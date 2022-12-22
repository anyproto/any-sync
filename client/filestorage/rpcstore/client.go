package rpcstore

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileblockstore"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileproto"
	_ "github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileproto/fileprotoerr"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/rpcerr"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/slice"
	"github.com/cheggaaa/mb/v3"
	"github.com/ipfs/go-cid"
	"go.uber.org/zap"
	"sync"
	"time"
)

var (
	ErrClientClosed = errors.New("file client closed")
)

const defaultMaxInFlightCIDs = 10

func newClient(ctx context.Context, s *service, peerId string, tq *mb.MB[*task]) (*client, error) {
	c := &client{
		peerId:     peerId,
		taskQueue:  tq,
		waitCIDs:   map[string]*task{},
		waitCIDCap: make(chan struct{}, defaultMaxInFlightCIDs),
		opLoopDone: make(chan struct{}),
		stat:       newStat(),
		s:          s,
	}
	if err := c.checkConnectivity(ctx); err != nil {
		return nil, err
	}
	var runCtx context.Context
	runCtx, c.opLoopCtxCancel = context.WithCancel(context.Background())
	go c.opLoop(runCtx)
	return c, nil
}

// client gets and executes tasks from taskQueue
// it has an internal queue for a waiting CIDs
type client struct {
	peerId          string
	spaceIds        []string
	taskQueue       *mb.MB[*task]
	blocksStream    fileproto.DRPCFile_GetBlocksClient
	blocksStreamMu  sync.Mutex
	waitCIDs        map[string]*task
	waitCIDCap      chan struct{}
	waitCIDMu       sync.Mutex
	opLoopDone      chan struct{}
	opLoopCtxCancel context.CancelFunc
	stat            *stat
	s               *service
}

// opLoop gets tasks from taskQueue
func (c *client) opLoop(ctx context.Context) {
	defer close(c.opLoopDone)
	c.waitCIDMu.Lock()
	spaceIds := c.spaceIds
	c.waitCIDMu.Unlock()
	cond := c.taskQueue.NewCond().WithFilter(func(t *task) bool {
		if slice.FindPos(t.denyPeerIds, c.peerId) != -1 {
			return false
		}
		if len(spaceIds) > 0 && slice.FindPos(spaceIds, t.spaceId) == -1 {
			return false
		}
		return true
	})
	for {
		t, err := cond.WithPriority(c.stat.Score()).WaitOne(ctx)
		if err != nil {
			return
		}
		t.peerId = c.peerId
		switch t.op {
		case taskOpGet:
			err = c.get(ctx, t)
		case taskOpDelete:
			err = c.delete(ctx, t)
		case taskOpPut:
			err = c.put(ctx, t)
		default:
			err = fmt.Errorf("unexpected task op type: %v", t.op)
		}
		if err != nil {
			t.err = err
			t.ready <- t
		}
	}
}

func (c *client) delete(ctx context.Context, t *task) (err error) {
	p, err := c.s.pool.Get(ctx, c.peerId)
	if err != nil {
		return
	}
	if _, err = fileproto.NewDRPCFileClient(p).DeleteBlocks(ctx, &fileproto.DeleteBlocksRequest{
		SpaceId: t.spaceId,
		Cid:     [][]byte{t.cid.Bytes()},
	}); err != nil {
		return rpcerr.Unwrap(err)
	}
	t.ready <- t
	c.stat.UpdateLastUsage()
	return
}

func (c *client) put(ctx context.Context, t *task) (err error) {
	p, err := c.s.pool.Get(ctx, c.peerId)
	if err != nil {
		return
	}
	st := time.Now()
	if _, err = fileproto.NewDRPCFileClient(p).PushBlock(ctx, &fileproto.PushBlockRequest{
		SpaceId: t.spaceId,
		Cid:     t.cid.Bytes(),
		Data:    t.data,
	}); err != nil {
		return rpcerr.Unwrap(err)
	}
	log.Debug("put cid", zap.String("cid", t.cid.String()))
	select {
	case t.ready <- t:
	case <-ctx.Done():
		return ctx.Err()
	}
	c.stat.Add(st, len(t.data))
	return
}

// get sends the get request to the stream and adds task to waiting list
func (c *client) get(ctx context.Context, t *task) (err error) {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.waitCIDCap <- struct{}{}:
	}
	c.waitCIDMu.Lock()
	t.startTime = time.Now()
	c.waitCIDs[t.cid.String()] = t
	c.waitCIDMu.Unlock()

	defer func() {
		if err != nil {
			c.waitCIDMu.Lock()
			delete(c.waitCIDs, t.cid.String())
			c.waitCIDMu.Unlock()
			<-c.waitCIDCap
		}
	}()

	bs, err := c.getStream(ctx)
	if err != nil {
		return
	}
	if err = bs.Send(&fileproto.GetBlockRequest{
		SpaceId: t.spaceId,
		Cid:     t.cid.Bytes(),
	}); err != nil {
		return rpcerr.Unwrap(err)
	}
	return
}

func (c *client) readStream(stream fileproto.DRPCFile_GetBlocksClient) {
	var err error
	defer func() {
		log.Info("readStream closed", zap.String("peerId", c.peerId), zap.Error(err))
		c.waitCIDMu.Lock()
		c.blocksStream = nil
		c.waitCIDMu.Unlock()
	}()
	for {
		var resp *fileproto.GetBlockResponse
		resp, err = stream.Recv()
		if err != nil {
			return
		}
		var t *task
		t, err = c.receiveCID(resp)
		if err != nil {
			log.Warn("cid receive error", zap.Error(err))
		} else {
			select {
			case t.ready <- t:
			case <-t.ctx.Done():
			}

		}
	}
}

// receiveCID handles stream response, finds cid in waiting list and sets data to task
func (c *client) receiveCID(resp *fileproto.GetBlockResponse) (t *task, err error) {
	_, rCid, err := cid.CidFromBytes(resp.Cid)
	if err != nil {
		return nil, fmt.Errorf("got invalid CID from node: %v", err)
	}
	c.waitCIDMu.Lock()
	defer c.waitCIDMu.Unlock()
	t, ok := c.waitCIDs[rCid.String()]
	if !ok {
		return nil, fmt.Errorf("got unexpected CID from node: %v", rCid.String())
	}
	switch resp.Code {
	case fileproto.CIDError_CIDErrorOk:
		t.data = resp.Data
		t.err = nil
	case fileproto.CIDError_CIDErrorNotFound:
		t.err = fileblockstore.ErrCIDNotFound
	default:
		t.err = fileblockstore.ErrCIDUnexpected
	}
	delete(c.waitCIDs, rCid.String())
	if t.err == nil {
		c.stat.Add(t.startTime, len(t.data))
	}
	<-c.waitCIDCap
	return
}

func (c *client) getStream(ctx context.Context) (fileproto.DRPCFile_GetBlocksClient, error) {
	c.blocksStreamMu.Lock()
	defer c.blocksStreamMu.Unlock()
	if c.blocksStream != nil {
		return c.blocksStream, nil
	}
	peer, err := c.s.pool.Dial(ctx, c.peerId)
	if err != nil {
		return nil, err
	}
	if c.blocksStream, err = fileproto.NewDRPCFileClient(peer).GetBlocks(context.Background()); err != nil {
		return nil, err
	}
	go c.readStream(c.blocksStream)
	return c.blocksStream, nil
}

func (c *client) checkConnectivity(ctx context.Context) (err error) {
	p, err := c.s.pool.Get(ctx, c.peerId)
	if err != nil {
		return
	}
	resp, err := fileproto.NewDRPCFileClient(p).Check(ctx, &fileproto.CheckRequest{})
	if err != nil {
		return
	}
	c.waitCIDMu.Lock()
	defer c.waitCIDMu.Unlock()
	c.spaceIds = resp.SpaceIds
	return
}

func (c *client) LastUsage() time.Time {
	return c.stat.LastUsage()
}

func (c *client) Close() error {
	// stop receiving tasks
	c.opLoopCtxCancel()
	<-c.opLoopDone
	c.blocksStreamMu.Lock()
	// close stream
	if c.blocksStream != nil {
		_ = c.blocksStream.CloseSend()
		c.blocksStream = nil
	}
	c.blocksStreamMu.Unlock()
	// cleanup waiting list
	c.waitCIDMu.Lock()
	for id, t := range c.waitCIDs {
		t.err = ErrClientClosed
		select {
		case t.ready <- t:
		case <-t.ctx.Done():
		}

		delete(c.waitCIDs, id)
	}
	c.waitCIDMu.Unlock()
	return nil
}

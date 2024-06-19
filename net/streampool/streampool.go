package streampool

import (
	"fmt"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	"golang.org/x/net/context"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/debugstat"
	"github.com/anyproto/any-sync/metric"
	"github.com/anyproto/any-sync/net"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/streampool/streamopener"
	"github.com/anyproto/any-sync/util/multiqueue"
)

type configGetter interface {
	GetStreamConfig() StreamConfig
}

type StreamSyncDelegate interface {
	// HandleMessage handles incoming message
	HandleMessage(ctx context.Context, peerId string, msg drpc.Message) (err error)
	// NewReadMessage creates new empty message for unmarshalling into it
	NewReadMessage() drpc.Message
	// GetQueue returns queue for outgoing messages
	GetQueue(peerId string) *multiqueue.Queue[drpc.Message]
	// RemoveQueue removes queue for outgoing messages
	RemoveQueue(peerId string) error
}

func NewStreamPool() StreamPool {
	return &streamPool{
		streamIdsByPeer: map[string][]uint32{},
		streams:         map[uint32]*stream{},
		opening:         map[string]*openingProcess{},
	}
}

// PeerGetter should dial or return cached peers
type PeerGetter func(ctx context.Context) (peers []peer.Peer, err error)

type MessageQueueId interface {
	MessageQueueId() string
	DrpcMessage() drpc.Message
}

// StreamPool keeps and read streams
type StreamPool interface {
	app.ComponentRunnable
	// AddStream adds new outgoing stream into the pool
	AddStream(stream drpc.Stream) (err error)
	// ReadStream adds new incoming stream and synchronously read it
	ReadStream(stream drpc.Stream) (err error)
	// Send sends a message to given peers. A stream will be opened if it is not cached before. Works async.
	Send(ctx context.Context, msg drpc.Message, target PeerGetter) (err error)
	// Broadcast sends a message to all peers with given tags. Works async.
	Broadcast(ctx context.Context, msg drpc.Message, tags ...string) (err error)
	// SetSyncDelegate registers sync delegate for handling incoming messages
	SetSyncDelegate(syncDelegate StreamSyncDelegate)
}

type streamPool struct {
	streamOpener    streamopener.StreamOpener
	syncDelegate    StreamSyncDelegate
	metric          metric.Metric
	statService     debugstat.StatService
	streamIdsByPeer map[string][]uint32
	streams         map[uint32]*stream
	opening         map[string]*openingProcess
	streamConfig    StreamConfig
	dial            *ExecPool
	mu              sync.Mutex
	writeQueueSize  int
	lastStreamId    uint32
}

func (s *streamPool) Init(a *app.App) (err error) {
	s.streamOpener = a.MustComponent(streamopener.CName).(streamopener.StreamOpener)
	s.metric, _ = a.Component(metric.CName).(metric.Metric)
	comp, ok := a.Component(debugstat.CName).(debugstat.StatService)
	if !ok {
		comp = debugstat.NewNoOp()
	}
	s.statService = comp
	s.streamConfig = a.MustComponent("config").(configGetter).GetStreamConfig()
	s.statService.AddProvider(s)
	return nil
}

func (s *streamPool) Name() (name string) {
	return CName
}

func (s *streamPool) Run(ctx context.Context) (err error) {
	s.dial = NewExecPool(s.streamConfig.DialQueueWorkers, s.streamConfig.DialQueueSize)
	s.dial.Run()
	return nil
}

func (s *streamPool) SetSyncDelegate(syncDelegate StreamSyncDelegate) {
	s.syncDelegate = syncDelegate
}

func (s *streamPool) ProvideStat() any {
	s.mu.Lock()
	var totalSize int64
	var stats []streamStat
	for _, st := range s.streams {
		cp := st.stats
		cp.TotalSize = cp.totalSize.Load()
		cp.MsgCount = int(cp.msgCount.Load())
		totalSize += cp.TotalSize
		stats = append(stats, cp)
	}
	s.mu.Unlock()
	slices.SortFunc(stats, func(first streamStat, second streamStat) int {
		if first.TotalSize > second.TotalSize {
			return -1
		} else if first.TotalSize == second.TotalSize {
			return 0
		} else {
			return 1
		}
	})
	return streamPoolStat{
		TotalSize: totalSize,
		Streams:   stats,
	}
}

func (s *streamPool) StatId() string {
	return CName
}

func (s *streamPool) StatType() string {
	return CName
}

type openingProcess struct {
	ch  chan struct{}
	err error
}

func (s *streamPool) ReadStream(drpcStream drpc.Stream) error {
	st, err := s.addStream(drpcStream)
	if err != nil {
		return err
	}
	go func() {
		st.writeLoop()
	}()
	return st.readLoop()
}

func (s *streamPool) AddStream(drpcStream drpc.Stream) error {
	st, err := s.addStream(drpcStream)
	if err != nil {
		return err
	}
	go func() {
		_ = st.readLoop()
	}()
	go func() {
		st.writeLoop()
	}()
	return nil
}

func (s *streamPool) addStream(drpcStream drpc.Stream) (*stream, error) {
	ctx := drpcStream.Context()
	peerId, err := peer.CtxPeerId(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastStreamId++
	streamId := s.lastStreamId
	st := &stream{
		peerId:       peerId,
		peerCtx:      ctx,
		stream:       drpcStream,
		pool:         s,
		streamId:     streamId,
		l:            log.With(zap.String("peerId", peerId), zap.Uint32("streamId", streamId)),
		syncDelegate: s.syncDelegate,
		stats:        newStreamStat(peerId),
	}
	st.queue = s.syncDelegate.GetQueue(peerId)
	if st.queue == nil {
		return nil, fmt.Errorf("no queue for peer %s", peerId)
	}
	s.streams[streamId] = st
	s.streamIdsByPeer[peerId] = append(s.streamIdsByPeer[peerId], streamId)
	return st, nil
}

func (s *streamPool) Send(ctx context.Context, msg drpc.Message, peerGetter PeerGetter) (err error) {
	return s.dial.TryAdd(func() {
		peers, dialErr := peerGetter(ctx)
		if dialErr != nil {
			log.InfoCtx(ctx, "can't get peers", zap.Error(dialErr))
		}
		for _, p := range peers {
			if e := s.sendOne(ctx, p, msg); e != nil {
				log.InfoCtx(ctx, "send peer error", zap.Error(e), zap.String("peerId", p.Id()))
			} else {
				log.DebugCtx(ctx, "send success", zap.String("peerId", p.Id()))
			}
		}
	})
}

func (s *streamPool) SendById(ctx context.Context, msg drpc.Message, peerIds ...string) (err error) {
	s.mu.Lock()
	var streamsByPeer [][]*stream
	for _, peerId := range peerIds {
		var streams []*stream
		for _, streamId := range s.streamIdsByPeer[peerId] {
			streams = append(streams, s.streams[streamId])
		}
		if len(streams) != 0 {
			streamsByPeer = append(streamsByPeer, streams)
		}
	}
	s.mu.Unlock()

	for _, streams := range streamsByPeer {
		for _, st := range streams {
			if e := st.write(msg); e != nil {
				st.l.Debug("sendById write error", zap.Error(e))
			} else {
				st.l.DebugCtx(ctx, "sendById success")
				return
			}
		}
	}
	if len(streamsByPeer) == 0 {
		return net.ErrUnableToConnect
	}
	return
}

func (s *streamPool) sendOne(ctx context.Context, p peer.Peer, msg drpc.Message) (err error) {
	// get all streams relates to peer
	streams, err := s.getStreams(ctx, p)
	if err != nil {
		return
	}
	for _, st := range streams {
		if err = st.write(msg); err != nil {
			st.l.InfoCtx(ctx, "sendOne write error", zap.Error(err), zap.Int("streams", len(streams)))
			// continue with next stream
			continue
		} else {
			st.l.DebugCtx(ctx, "sendOne success")
			// stop sending on success
			break
		}
	}
	return
}

func (s *streamPool) getStreams(ctx context.Context, p peer.Peer) (streams []*stream, err error) {
	s.mu.Lock()
	// check cached streams
	streamIds := s.streamIdsByPeer[p.Id()]
	for _, streamId := range streamIds {
		streams = append(streams, s.streams[streamId])
	}
	var op *openingProcess
	// no cached streams found
	if len(streams) == 0 {
		// start opening process
		op = s.openStream(ctx, p)
	}
	s.mu.Unlock()

	// not empty openingCh means we should wait for the stream opening and try again
	if op != nil {
		select {
		case <-op.ch:
			if op.err != nil {
				return nil, op.err
			}
			return s.getStreams(ctx, p)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return streams, nil
}

func (s *streamPool) openStream(ctx context.Context, p peer.Peer) *openingProcess {
	if op, ok := s.opening[p.Id()]; ok {
		// already have an opening process for this stream - return channel
		return op
	}
	op := &openingProcess{
		ch: make(chan struct{}),
	}
	s.opening[p.Id()] = op
	go func() {
		// start stream opening in separate goroutine to avoid lock whole pool
		defer func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			close(op.ch)
			delete(s.opening, p.Id())
		}()
		// in case there was no peerId in context
		ctx := peer.CtxWithPeerId(ctx, p.Id())
		// open new stream and add to pool
		st, _, err := s.streamOpener.OpenStream(ctx, p)
		if err != nil {
			op.err = err
			return
		}
		if err = s.AddStream(st); err != nil {
			op.err = nil
			return
		}
	}()
	return op
}

func (s *streamPool) Broadcast(ctx context.Context, msg drpc.Message, tags ...string) (err error) {
	s.mu.Lock()
	var streams []*stream
	for _, peerStreams := range s.streamIdsByPeer {
		if len(peerStreams) > 0 {
			streams = append(streams, s.streams[peerStreams[0]])
		}
	}
	s.mu.Unlock()

	for _, st := range streams {
		if e := st.write(msg); e != nil {
			st.l.InfoCtx(ctx, "broadcast write error", zap.Error(e))
		} else {
			st.l.DebugCtx(ctx, "broadcast success")
		}
	}
	return
}

func (s *streamPool) removeStream(streamId uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.streams[streamId]
	if st == nil {
		log.Fatal("removeStream: stream does not exist", zap.Uint32("streamId", streamId))
	}
	removeStream(s.streamIdsByPeer, st.peerId, streamId)
	delete(s.streams, streamId)
	st.l.Debug("stream removed", zap.String("peerId", st.peerId))
}

func (s *streamPool) Close(ctx context.Context) (err error) {
	s.statService.RemoveProvider(s)
	return s.dial.Close()
}

func removeStream(m map[string][]uint32, key string, streamId uint32) {
	streamIds := m[key]
	idx := slices.Index(streamIds, streamId)
	if idx == -1 {
		log.Fatal("removeStream: streamId does not exist", zap.Uint32("streamId", streamId))
	}
	streamIds = slices.Delete(streamIds, idx, idx+1)
	if len(streamIds) == 0 {
		delete(m, key)
	} else {
		m[key] = streamIds
	}
}

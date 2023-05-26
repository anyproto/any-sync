package treemanager

import (
	"context"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/streampool"
	"go.uber.org/zap"
	"sync"
	"time"
)

type executor struct {
	pool *streampool.ExecPool
	objs map[string]struct{}
	sync.Mutex
}

func newExecutor(workers, size int) *executor {
	return &executor{
		pool: streampool.NewExecPool(workers, size),
		objs: map[string]struct{}{},
	}
}

func (e *executor) tryAdd(id string, action func()) (err error) {
	if _, exists := e.objs[id]; exists {
		return nil
	}
	e.Lock()
	defer e.Unlock()
	e.objs[id] = struct{}{}
	return e.pool.TryAdd(func() {
		action()
		e.Lock()
		defer e.Unlock()
		delete(e.objs, id)
	})
}

func (e *executor) close() {
	e.pool.Close()
}

type treeSyncer struct {
	sync.Mutex
	log          logger.CtxLogger
	size         int
	requests     int
	spaceId      string
	timeout      time.Duration
	requestPools map[string]*executor
	headPools    map[string]*executor
	treeManager  TreeManager
	isRunning    bool
}

func NewTreeSyncer(spaceId string, timeout time.Duration, concurrentReqs int, treeManager TreeManager, log logger.CtxLogger) TreeSyncer {
	return &treeSyncer{
		log:          log,
		requests:     concurrentReqs,
		spaceId:      spaceId,
		timeout:      timeout,
		requestPools: map[string]*executor{},
		headPools:    map[string]*executor{},
		treeManager:  treeManager,
	}
}

func (t *treeSyncer) Init() {
	t.Lock()
	defer t.Unlock()
	t.isRunning = true
}

func (t *treeSyncer) SyncAll(ctx context.Context, peerId string, existing, missing []string) error {
	t.Lock()
	defer t.Unlock()
	if !t.isRunning {
		return nil
	}
	reqExec, exists := t.requestPools[peerId]
	if !exists {
		reqExec = newExecutor(t.requests, t.size)
		t.requestPools[peerId] = reqExec
	}
	headExec, exists := t.headPools[peerId]
	if !exists {
		headExec = newExecutor(1, t.size)
		t.requestPools[peerId] = headExec
	}
	for _, id := range existing {
		err := headExec.tryAdd(id, func() {
			t.updateTree(peerId, id)
		})
		if err != nil {
			t.log.Error("failed to add to head queue", zap.Error(err))
		}
	}
	for _, id := range missing {
		err := reqExec.tryAdd(id, func() {
			t.requestTree(peerId, id)
		})
		if err != nil {
			t.log.Error("failed to add to request queue", zap.Error(err))
		}
	}
	return nil
}

func (t *treeSyncer) requestTree(peerId, id string) {
	log := t.log.With(zap.String("treeId", id))
	ctx := peer.CtxWithPeerId(context.Background(), peerId)
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	_, err := t.treeManager.GetTree(ctx, t.spaceId, id)
	if err != nil {
		log.WarnCtx(ctx, "can't load missing tree", zap.Error(err))
	} else {
		log.DebugCtx(ctx, "loaded missing tree")
	}
}

func (t *treeSyncer) updateTree(peerId, id string) {
	log := t.log.With(zap.String("treeId", id))
	ctx := peer.CtxWithPeerId(context.Background(), peerId)
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	tr, err := t.treeManager.GetTree(ctx, t.spaceId, id)
	if err != nil {
		log.WarnCtx(ctx, "can't load existing tree", zap.Error(err))
		return
	}
	syncTree, ok := tr.(synctree.SyncTree)
	if !ok {
		log.WarnCtx(ctx, "not a sync tree")
	}
	if err = syncTree.SyncWithPeer(ctx, peerId); err != nil {
		log.WarnCtx(ctx, "synctree.SyncWithPeer error", zap.Error(err))
	} else {
		log.DebugCtx(ctx, "success synctree.SyncWithPeer")
	}
}

func (t *treeSyncer) Close() error {
	t.Lock()
	defer t.Unlock()
	t.isRunning = false
	for _, pool := range t.headPools {
		pool.close()
	}
	for _, pool := range t.requestPools {
		pool.close()
	}
	return nil
}

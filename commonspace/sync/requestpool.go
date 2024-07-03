package sync

import (
	"context"
	"sync"
)

type RequestPool interface {
	TryTake(peerId, objectId string) bool
	Release(peerId, objectId string)
	QueueRequestAction(peerId, objectId string, action func(ctx context.Context), remove func()) (err error)
	Close()
}

type requestPool struct {
	mu        sync.Mutex
	peerGuard *guard
	pools     map[string]*tryAddQueue
	ctx       context.Context
	cancel    context.CancelFunc
	isClosed  bool
}

func NewRequestPool() RequestPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &requestPool{
		ctx:       ctx,
		cancel:    cancel,
		pools:     make(map[string]*tryAddQueue),
		peerGuard: newGuard(0),
	}
}

func (rp *requestPool) TryTake(peerId, objectId string) bool {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	if rp.isClosed {
		return false
	}

	return rp.peerGuard.TryTake(fullId(peerId, objectId))
}

func (rp *requestPool) Release(peerId, objectId string) {
	rp.peerGuard.Release(fullId(peerId, objectId))
}

func (rp *requestPool) QueueRequestAction(peerId, objectId string, action func(ctx context.Context), remove func()) (err error) {
	rp.mu.Lock()
	if rp.isClosed {
		rp.mu.Unlock()
		return nil
	}
	var (
		pool   *tryAddQueue
		exists bool
	)
	pool, exists = rp.pools[peerId]
	if !exists {
		pool = newTryAddQueue(10, 100)
		rp.pools[peerId] = pool
		pool.Run()
	}
	rp.mu.Unlock()
	var wrappedAction func()
	wrappedAction = func() {
		if !rp.TryTake(peerId, objectId) {
			return
		}
		action(rp.ctx)
		rp.Release(peerId, objectId)
	}
	pool.Replace(objectId, wrappedAction, remove)
	return nil
}

func (rp *requestPool) Close() {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.isClosed = true
	for _, pool := range rp.pools {
		_ = pool.Close()
	}
}

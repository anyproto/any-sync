package globalsync

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/util/periodicsync"
)

type RequestPool interface {
	TryTake(peerId, objectId string) bool
	Release(peerId, objectId string)
	Run()
	QueueRequestAction(peerId, objectId string, action func(ctx context.Context), remove func()) (err error)
	Close()
}

type requestPool struct {
	mu           sync.Mutex
	peerGuard    *Guard
	pools        map[string]*tryAddQueue
	periodicLoop periodicsync.PeriodicSync
	closePeriod  time.Duration
	gcPeriod     time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
	isClosed     bool
}

func NewRequestPool(closePeriod, gcPeriod time.Duration) RequestPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &requestPool{
		ctx:         ctx,
		cancel:      cancel,
		closePeriod: closePeriod,
		gcPeriod:    gcPeriod,
		pools:       make(map[string]*tryAddQueue),
		peerGuard:   NewGuard(),
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

func (rp *requestPool) Run() {
	rp.periodicLoop = periodicsync.NewPeriodicSyncDuration(rp.gcPeriod, time.Minute, rp.gc, log)
	rp.periodicLoop.Run()
}

func (rp *requestPool) gc(ctx context.Context) error {
	rp.mu.Lock()
	var poolsToClose []*tryAddQueue
	tm := time.Now()
	for id, pool := range rp.pools {
		if pool.ShouldClose(tm, rp.closePeriod) {
			delete(rp.pools, id)
			log.Debug("closing pool", zap.String("peerId", id))
			poolsToClose = append(poolsToClose, pool)
		}
	}
	rp.mu.Unlock()
	for _, pool := range poolsToClose {
		_ = pool.Close()
	}
	return nil
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
	rp.periodicLoop.Close()
	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.isClosed = true
	for _, pool := range rp.pools {
		_ = pool.Close()
	}
}

package syncqueues

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/util/periodicsync"
)

type ActionPool interface {
	Run()
	Add(peerId, objectId string, action func(ctx context.Context), remove func())
	Close()
}

type actionPool struct {
	mu           sync.Mutex
	peerGuard    *Guard
	queues       map[string]*replaceableQueue
	periodicLoop periodicsync.PeriodicSync
	closePeriod  time.Duration
	gcPeriod     time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
	openFunc     func(peerId string) *replaceableQueue
	isClosed     bool
}

func NewActionPool(closePeriod, gcPeriod time.Duration, openFunc func(peerId string) *replaceableQueue) ActionPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &actionPool{
		ctx:         ctx,
		cancel:      cancel,
		closePeriod: closePeriod,
		gcPeriod:    gcPeriod,
		openFunc:    openFunc,
		queues:      make(map[string]*replaceableQueue),
		peerGuard:   NewGuard(),
	}
}

func (rp *actionPool) tryTake(peerId, objectId string) bool {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	if rp.isClosed {
		return false
	}

	return rp.peerGuard.TryTake(fullId(peerId, objectId))
}

func (rp *actionPool) release(peerId, objectId string) {
	rp.peerGuard.Release(fullId(peerId, objectId))
}

func (rp *actionPool) Run() {
	rp.periodicLoop = periodicsync.NewPeriodicSyncDuration(rp.gcPeriod, time.Minute, rp.gc, log)
	rp.periodicLoop.Run()
}

func (rp *actionPool) gc(ctx context.Context) error {
	rp.mu.Lock()
	var queuesToClose []*replaceableQueue
	tm := time.Now()
	for id, queue := range rp.queues {
		if queue.ShouldClose(tm, rp.closePeriod) {
			delete(rp.queues, id)
			log.Debug("closing queue", zap.String("peerId", id))
			queuesToClose = append(queuesToClose, queue)
		}
	}
	rp.mu.Unlock()
	for _, queue := range queuesToClose {
		_ = queue.Close()
	}
	return nil
}

func (rp *actionPool) Add(peerId, objectId string, action func(ctx context.Context), remove func()) {
	rp.mu.Lock()
	if rp.isClosed {
		rp.mu.Unlock()
		return
	}
	var (
		queue  *replaceableQueue
		exists bool
	)
	queue, exists = rp.queues[peerId]
	if !exists {
		queue = rp.openFunc(peerId)
		rp.queues[peerId] = queue
		queue.Run()
	}
	rp.mu.Unlock()
	var wrappedAction func()
	wrappedAction = func() {
		// this prevents cases when two simultaneous requests are sent at the same time
		if !rp.tryTake(peerId, objectId) {
			return
		}
		action(rp.ctx)
		rp.release(peerId, objectId)
	}
	queue.Replace(objectId, wrappedAction, remove)
}

func (rp *actionPool) Close() {
	rp.periodicLoop.Close()
	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.isClosed = true
	for _, queue := range rp.queues {
		_ = queue.Close()
	}
}

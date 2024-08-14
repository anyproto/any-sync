package syncqueues

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cheggaaa/mb/v3"
	"go.uber.org/zap"
)

type entry struct {
	call     func()
	onRemove func()
	cnt      uint64
}

func newReplaceableQueue(workers, maxSize int) *replaceableQueue {
	ctx, cancel := context.WithCancel(context.Background())
	ss := &replaceableQueue{
		ctx:     ctx,
		cancel:  cancel,
		workers: workers,
		batch:   mb.New[string](maxSize),
		entries: map[string]entry{},
	}
	return ss
}

type replaceableQueue struct {
	ctx        context.Context
	cancel     context.CancelFunc
	workers    int
	cnt        atomic.Uint64
	lastServed time.Time
	entries    map[string]entry
	batch      *mb.MB[string]
	mx         sync.Mutex
}

func (rp *replaceableQueue) Replace(id string, call, remove func()) {
	curCnt := rp.cnt.Load()
	rp.cnt.Add(1)
	rp.mx.Lock()
	if prevEntry, ok := rp.entries[id]; ok {
		rp.entries[id] = entry{
			call:     call,
			onRemove: remove,
			cnt:      curCnt,
		}
		rp.mx.Unlock()
		prevEntry.onRemove()
		return
	}
	ent := entry{
		call:     call,
		onRemove: remove,
		cnt:      curCnt,
	}
	rp.entries[id] = ent
	rp.mx.Unlock()
	err := rp.batch.TryAdd(id)
	if err != nil {
		rp.mx.Lock()
		curEntry := rp.entries[id]
		if curEntry.cnt == curCnt {
			delete(rp.entries, id)
		}
		rp.mx.Unlock()
		if ent.onRemove != nil {
			ent.onRemove()
		}
	}
}

func (rp *replaceableQueue) Run() {
	for i := 0; i < rp.workers; i++ {
		go rp.callLoop()
	}
}

func (rp *replaceableQueue) callLoop() {
	for {
		id, err := rp.batch.WaitOne(rp.ctx)
		if err != nil {
			log.Debug("close call loop", zap.Error(err))
			return
		}
		rp.mx.Lock()
		rp.lastServed = time.Now()
		curEntry := rp.entries[id]
		delete(rp.entries, id)
		rp.mx.Unlock()
		if curEntry.call != nil {
			curEntry.call()
			if curEntry.onRemove != nil {
				curEntry.onRemove()
			}
		}
	}
}

func (rp *replaceableQueue) ShouldClose(curTime time.Time, timeout time.Duration) bool {
	rp.mx.Lock()
	defer rp.mx.Unlock()
	return curTime.Sub(rp.lastServed) > timeout && rp.batch.Len() == 0
}

func (rp *replaceableQueue) Close() (err error) {
	rp.cancel()
	return rp.batch.Close()
}

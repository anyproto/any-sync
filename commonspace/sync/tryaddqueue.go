package sync

import (
	"context"
	"sync"

	"github.com/cheggaaa/mb/v3"
	"go.uber.org/zap"
)

type entry struct {
	call     func()
	onRemove func()
}

func newTryAddQueue(workers, maxSize int) *tryAddQueue {
	ctx, cancel := context.WithCancel(context.Background())
	ss := &tryAddQueue{
		ctx:     ctx,
		cancel:  cancel,
		workers: workers,
		batch:   mb.New[string](maxSize),
		entries: map[string]entry{},
	}
	return ss
}

type tryAddQueue struct {
	ctx     context.Context
	cancel  context.CancelFunc
	workers int

	entries map[string]entry
	batch   *mb.MB[string]
	mx      sync.Mutex
}

func (rp *tryAddQueue) Replace(id string, call, remove func()) {
	rp.mx.Lock()
	if prevEntry, ok := rp.entries[id]; ok {
		rp.entries[id] = entry{
			call:     call,
			onRemove: remove,
		}
		rp.mx.Unlock()
		prevEntry.onRemove()
		return
	}
	rp.entries[id] = entry{
		call:     call,
		onRemove: remove,
	}
	rp.mx.Unlock()
	err := rp.batch.TryAdd(id)
	if err != nil {
		rp.mx.Lock()
		curEntry := rp.entries[id]
		delete(rp.entries, id)
		rp.mx.Unlock()
		if curEntry.onRemove != nil {
			curEntry.onRemove()
		}
	}
}

func (rp *tryAddQueue) TryAdd(id string, call, remove func()) bool {
	rp.mx.Lock()
	if _, ok := rp.entries[id]; ok {
		rp.mx.Unlock()
		return false
	}
	rp.entries[id] = entry{
		call:     call,
		onRemove: remove,
	}
	rp.mx.Unlock()
	if err := rp.batch.TryAdd(id); err != nil {
		return false
	}
	return true
}

func (rp *tryAddQueue) Run() {
	for i := 0; i < rp.workers; i++ {
		go rp.sendLoop()
	}
}

func (rp *tryAddQueue) sendLoop() {
	for {
		id, err := rp.batch.WaitOne(rp.ctx)
		if err != nil {
			log.Debug("close send loop", zap.Error(err))
			return
		}
		rp.mx.Lock()
		curEntry := rp.entries[id]
		delete(rp.entries, id)
		rp.mx.Unlock()
		if curEntry.call != nil {
			curEntry.call()
			curEntry.onRemove()
		}
	}
}

func (rp *tryAddQueue) Close() (err error) {
	rp.cancel()
	return rp.batch.Close()
}

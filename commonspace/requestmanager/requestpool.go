package requestmanager

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

// newRequestPool creates new requestPool
// workers - how many processes will execute tasks
// maxSize - limit for queue size
func newRequestPool(workers, maxSize int) *requestPool {
	ctx, cancel := context.WithCancel(context.Background())
	ss := &requestPool{
		ctx:     ctx,
		cancel:  cancel,
		workers: workers,
		batch:   mb.New[string](maxSize),
		entries: map[string]entry{},
	}
	return ss
}

// requestPool needed for parallel execution of the incoming send tasks
type requestPool struct {
	ctx     context.Context
	cancel  context.CancelFunc
	workers int

	entries map[string]entry
	batch   *mb.MB[string]
	mx      sync.Mutex
}

func (rp *requestPool) TryAdd(id string, call, remove func()) (err error) {
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
	err = rp.batch.TryAdd(id)
	if err != nil {
		rp.mx.Lock()
		curEntry := rp.entries[id]
		delete(rp.entries, id)
		rp.mx.Unlock()
		if curEntry.onRemove != nil {
			curEntry.onRemove()
		}
	}
	return
}

func (rp *requestPool) Run() {
	for i := 0; i < rp.workers; i++ {
		go rp.sendLoop()
	}
}

func (rp *requestPool) sendLoop() {
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

func (rp *requestPool) Close() (err error) {
	rp.cancel()
	return rp.batch.Close()
}

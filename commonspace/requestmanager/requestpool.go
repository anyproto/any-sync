package requestmanager

import (
	"context"
	"sync"

	"github.com/cheggaaa/mb/v3"
	"go.uber.org/zap"
)

// NewRequestPool creates new RequestPool
// workers - how many processes will execute tasks
// maxSize - limit for queue size
func NewRequestPool(workers, maxSize int) *RequestPool {
	ctx, cancel := context.WithCancel(context.Background())
	ss := &RequestPool{
		ctx:     ctx,
		cancel:  cancel,
		workers: workers,
		batch:   mb.New[string](maxSize),
		entries: map[string]func(){},
	}
	return ss
}

// RequestPool needed for parallel execution of the incoming send tasks
type RequestPool struct {
	ctx     context.Context
	cancel  context.CancelFunc
	workers int

	entries map[string]func()
	batch   *mb.MB[string]
	mx      sync.Mutex
}

func (rp *RequestPool) TryAdd(id string, f func()) (err error) {
	rp.mx.Lock()
	if _, ok := rp.entries[id]; ok {
		rp.entries[id] = f
		rp.mx.Unlock()
		return
	}
	rp.entries[id] = f
	rp.mx.Unlock()
	err = rp.batch.TryAdd(id)
	if err != nil {
		rp.mx.Lock()
		delete(rp.entries, id)
		rp.mx.Unlock()
	}
	return
}

func (rp *RequestPool) Run() {
	for i := 0; i < rp.workers; i++ {
		go rp.sendLoop()
	}
}

func (rp *RequestPool) sendLoop() {
	for {
		id, err := rp.batch.WaitOne(rp.ctx)
		if err != nil {
			log.Debug("close send loop", zap.Error(err))
			return
		}
		rp.mx.Lock()
		if f, ok := rp.entries[id]; ok {
			delete(rp.entries, id)
			rp.mx.Unlock()
			f()
		} else {
			rp.mx.Unlock()
		}
	}
}

func (rp *RequestPool) Close() (err error) {
	rp.cancel()
	return rp.batch.Close()
}

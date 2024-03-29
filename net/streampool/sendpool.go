package streampool

import (
	"context"
	"github.com/cheggaaa/mb/v3"
	"go.uber.org/zap"
)

// NewExecPool creates new ExecPool
// workers - how many processes will execute tasks
// maxSize - limit for queue size
func NewExecPool(workers, maxSize int) *ExecPool {
	ctx, cancel := context.WithCancel(context.Background())
	ss := &ExecPool{
		ctx:     ctx,
		cancel:  cancel,
		workers: workers,
		batch:   mb.New[func()](maxSize),
	}
	return ss
}

// ExecPool needed for parallel execution of the incoming send tasks
type ExecPool struct {
	ctx     context.Context
	cancel  context.CancelFunc
	workers int
	batch   *mb.MB[func()]
}

func (ss *ExecPool) Add(ctx context.Context, f ...func()) (err error) {
	return ss.batch.Add(ctx, f...)
}

func (ss *ExecPool) TryAdd(f ...func()) (err error) {
	return ss.batch.TryAdd(f...)
}

func (ss *ExecPool) Run() {
	for i := 0; i < ss.workers; i++ {
		go ss.sendLoop()
	}
}

func (ss *ExecPool) sendLoop() {
	for {
		f, err := ss.batch.WaitOne(ss.ctx)
		if err != nil {
			log.Debug("close send loop", zap.Error(err))
			return
		}
		f()
	}
}

func (ss *ExecPool) Close() (err error) {
	ss.cancel()
	return ss.batch.Close()
}

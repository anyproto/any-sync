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
	ss := &ExecPool{
		batch: mb.New[func()](maxSize),
	}
	for i := 0; i < workers; i++ {
		go ss.sendLoop()
	}
	return ss
}

// ExecPool needed for parallel execution of the incoming send tasks
type ExecPool struct {
	batch *mb.MB[func()]
}

func (ss *ExecPool) Add(ctx context.Context, f ...func()) (err error) {
	return ss.batch.Add(ctx, f...)
}

func (ss *ExecPool) TryAdd(f ...func()) (err error) {
	return ss.batch.TryAdd(f...)
}

func (ss *ExecPool) sendLoop() {
	for {
		f, err := ss.batch.WaitOne(context.Background())
		if err != nil {
			log.Debug("close send loop", zap.Error(err))
			return
		}
		f()
	}
}

func (ss *ExecPool) Close() (err error) {
	return ss.batch.Close()
}

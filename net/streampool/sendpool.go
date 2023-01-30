package streampool

import (
	"context"
	"github.com/cheggaaa/mb/v3"
	"go.uber.org/zap"
)

// newExecPool creates new execPool
// workers - how many processes will execute tasks
// maxSize - limit for queue size
func newExecPool(workers, maxSize int) *execPool {
	ss := &execPool{
		batch: mb.New[func()](maxSize),
	}
	for i := 0; i < workers; i++ {
		go ss.sendLoop()
	}
	return ss
}

// execPool needed for parallel execution of the incoming send tasks
type execPool struct {
	batch *mb.MB[func()]
}

func (ss *execPool) Add(ctx context.Context, f ...func()) (err error) {
	return ss.batch.Add(ctx, f...)
}

func (ss *execPool) sendLoop() {
	for {
		f, err := ss.batch.WaitOne(context.Background())
		if err != nil {
			log.Debug("close send loop", zap.Error(err))
			return
		}
		f()
	}
}

func (ss *execPool) Close() (err error) {
	return ss.batch.Close()
}

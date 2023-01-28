package streampool

import (
	"context"
	"github.com/cheggaaa/mb/v3"
	"go.uber.org/zap"
)

// newStreamSender creates new sendPool
// workers - how many processes will execute tasks
// maxSize - limit for queue size
func newStreamSender(workers, maxSize int) *sendPool {
	ss := &sendPool{
		batch: mb.New[func()](maxSize),
	}
	for i := 0; i < workers; i++ {
		go ss.sendLoop()
	}
	return ss
}

// sendPool needed for parallel execution of the incoming send tasks
type sendPool struct {
	batch *mb.MB[func()]
}

func (ss *sendPool) Add(ctx context.Context, f ...func()) (err error) {
	return ss.batch.Add(ctx, f...)
}

func (ss *sendPool) sendLoop() {
	for {
		f, err := ss.batch.WaitOne(context.Background())
		if err != nil {
			log.Debug("close send loop", zap.Error(err))
			return
		}
		f()
	}
}

func (ss *sendPool) Close() (err error) {
	return ss.batch.Close()
}

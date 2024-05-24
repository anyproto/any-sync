package streampool

import (
	"context"

	"github.com/cheggaaa/mb/v3"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/logger"
)

var log = logger.NewNamed("common.net.streampool")

// NewExecPool creates new ExecPool
// workers - how many processes will execute tasks
// maxSize - limit for queue size
func NewExecPool(workers, maxSize int) *ExecPool {
	ctx, cancel := context.WithCancel(context.Background())
	ss := &ExecPool{
		ctx:     ctx,
		cancel:  cancel,
		workers: workers,
		Batch:   mb.New[func()](maxSize),
	}
	return ss
}

// ExecPool needed for parallel execution of the incoming send tasks
type ExecPool struct {
	ctx     context.Context
	cancel  context.CancelFunc
	workers int
	Batch   *mb.MB[func()] //should be private
}

func (ss *ExecPool) Add(ctx context.Context, f ...func()) (err error) {
	return ss.Batch.Add(ctx, f...)
}

func (ss *ExecPool) TryAdd(f ...func()) (err error) {
	return ss.Batch.TryAdd(f...)
}

func (ss *ExecPool) Run() {
	for i := 0; i < ss.workers; i++ {
		go ss.sendLoop()
	}
}

func (ss *ExecPool) sendLoop() {
	for {
		f, err := ss.Batch.WaitOne(ss.ctx)
		if err != nil {
			log.Debug("close send loop", zap.Error(err))
			return
		}
		f()
	}
}

func (ss *ExecPool) Close() (err error) {
	ss.cancel()
	return ss.Batch.Close()
}

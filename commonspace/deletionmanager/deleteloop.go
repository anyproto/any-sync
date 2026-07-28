package deletionmanager

import (
	"context"
	"time"

	"go.uber.org/zap"
)

const (
	deleteLoopInterval = time.Second * 20
	deleteCloseTimeout = time.Second * 10
)

type deleteLoop struct {
	deleteCtx    context.Context
	deleteCancel context.CancelFunc
	deleteChan   chan struct{}
	deleteFunc   func(ctx context.Context)
	loopDone     chan struct{}
	closeTimeout time.Duration
}

func newDeleteLoop(deleteFunc func(ctx context.Context)) *deleteLoop {
	ctx, cancel := context.WithCancel(context.Background())
	return &deleteLoop{
		deleteCtx:    ctx,
		deleteCancel: cancel,
		deleteChan:   make(chan struct{}, 1),
		deleteFunc:   deleteFunc,
		loopDone:     make(chan struct{}),
		closeTimeout: deleteCloseTimeout,
	}
}

func (dl *deleteLoop) Run() {
	go dl.loop()
}

func (dl *deleteLoop) loop() {
	defer close(dl.loopDone)
	dl.deleteFunc(dl.deleteCtx)
	ticker := time.NewTicker(deleteLoopInterval)
	defer ticker.Stop()
	for {
		select {
		case <-dl.deleteCtx.Done():
			return
		case <-dl.deleteChan:
			dl.deleteFunc(dl.deleteCtx)
			ticker.Reset(deleteLoopInterval)
		case <-ticker.C:
			dl.deleteFunc(dl.deleteCtx)
		}
	}
}

func (dl *deleteLoop) notify() {
	select {
	case dl.deleteChan <- struct{}{}:
	default:
	}
}

// Close cancels the delete context and waits for the loop to exit. The wait is bounded:
// deleteFunc calls into treemanager implementations that may not honour ctx, and blocking
// here forever wedges the whole app.Close.
func (dl *deleteLoop) Close(ctx context.Context) {
	dl.deleteCancel()
	timer := time.NewTimer(dl.closeTimeout)
	defer timer.Stop()
	select {
	case <-dl.loopDone:
	case <-ctx.Done():
		log.WarnCtx(ctx, "delete loop close interrupted, delete is still in flight", zap.Error(ctx.Err()))
	case <-timer.C:
		log.WarnCtx(ctx, "delete loop close timed out, delete is still in flight")
	}
}

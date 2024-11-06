package deletionmanager

import (
	"context"
	"time"
)

const deleteLoopInterval = time.Second * 20

type deleteLoop struct {
	deleteCtx    context.Context
	deleteCancel context.CancelFunc
	deleteChan   chan struct{}
	deleteFunc   func(ctx context.Context)
	loopDone     chan struct{}
}

func newDeleteLoop(deleteFunc func(ctx context.Context)) *deleteLoop {
	ctx, cancel := context.WithCancel(context.Background())
	return &deleteLoop{
		deleteCtx:    ctx,
		deleteCancel: cancel,
		deleteChan:   make(chan struct{}, 1),
		deleteFunc:   deleteFunc,
		loopDone:     make(chan struct{}),
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

func (dl *deleteLoop) Close() {
	dl.deleteCancel()
	<-dl.loopDone
}

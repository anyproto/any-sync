package settingsdocument

import (
	"context"
)

type deleteLoop struct {
	deleteCtx    context.Context
	deleteCancel context.CancelFunc
	deleteChan   chan struct{}
	deleteFunc   func()
	loopDone     chan struct{}
}

func newDeleteLoop(deleteFunc func()) *deleteLoop {
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
	dl.deleteFunc()
	for {
		select {
		case <-dl.deleteCtx.Done():
			return
		case <-dl.deleteChan:
			dl.deleteFunc()
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

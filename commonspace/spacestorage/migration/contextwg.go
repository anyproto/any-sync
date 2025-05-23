package migration

import (
	"context"
	"sync"
)

type contextWaitGroup struct {
	ctx context.Context
	wg  sync.WaitGroup
}

func newContextWaitGroup(ctx context.Context) *contextWaitGroup {
	return &contextWaitGroup{
		ctx: ctx,
	}
}

func (cwg *contextWaitGroup) Add(delta int) {
	cwg.wg.Add(delta)
}

func (cwg *contextWaitGroup) Done() {
	cwg.wg.Done()
}

func (cwg *contextWaitGroup) Wait() error {
	doneCh := make(chan struct{})
	go func() {
		cwg.wg.Wait()
		close(doneCh)
	}()
	select {
	case <-doneCh:
		return nil
	case <-cwg.ctx.Done():
		return cwg.ctx.Err()
	}
}

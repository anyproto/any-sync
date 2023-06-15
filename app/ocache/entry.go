package ocache

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

type entryState int

const (
	entryStateLoading = iota
	entryStateActive
	entryStateClosing
	entryStateClosed
)

type entry struct {
	id        string
	state     entryState
	lastUsage time.Time
	load      chan struct{}
	loadErr   error
	value     Object
	close     chan struct{}
	mx        sync.Mutex
	cancel    context.CancelFunc
}

func newEntry(id string, value Object, state entryState) *entry {
	return &entry{
		id:        id,
		load:      make(chan struct{}),
		lastUsage: time.Now(),
		state:     state,
		value:     value,
	}
}

func (e *entry) isActive() bool {
	e.mx.Lock()
	defer e.mx.Unlock()
	return e.state == entryStateActive
}

func (e *entry) isClosing() bool {
	e.mx.Lock()
	defer e.mx.Unlock()
	return e.state == entryStateClosed || e.state == entryStateClosing
}

func (e *entry) setCancel(cancel context.CancelFunc) {
	e.mx.Lock()
	defer e.mx.Unlock()
	e.cancel = cancel
}

func (e *entry) cancelLoad() {
	e.mx.Lock()
	defer e.mx.Unlock()
	if e.cancel != nil {
		e.cancel()
	}
}

func (e *entry) waitLoad(ctx context.Context, id string) (value Object, err error) {
	select {
	case <-ctx.Done():
		log.DebugCtx(ctx, "ctx done while waiting on object load", zap.String("id", id))
		return nil, ctx.Err()
	case <-e.load:
		return e.value, e.loadErr
	}
}

func (e *entry) waitClose(ctx context.Context, id string) (res bool, err error) {
	e.mx.Lock()
	switch e.state {
	case entryStateClosing:
		waitCh := e.close
		e.mx.Unlock()
		select {
		case <-ctx.Done():
			log.DebugCtx(ctx, "ctx done while waiting on object close", zap.String("id", id))
			return false, ctx.Err()
		case <-waitCh:
			return true, nil
		}
	case entryStateClosed:
		e.mx.Unlock()
		return true, nil
	default:
		e.mx.Unlock()
		return false, nil
	}
}

func (e *entry) setClosing(wait bool) (prevState, curState entryState) {
	e.mx.Lock()
	prevState = e.state
	curState = e.state
	if e.state == entryStateClosing {
		waitCh := e.close
		e.mx.Unlock()
		if !wait {
			return
		}
		<-waitCh
		e.mx.Lock()
	}
	if e.state != entryStateClosed {
		e.state = entryStateClosing
		e.close = make(chan struct{})
	}
	curState = e.state
	e.mx.Unlock()
	return
}

func (e *entry) setActive(chClose bool) {
	e.mx.Lock()
	defer e.mx.Unlock()
	if chClose {
		close(e.close)
	}
	e.state = entryStateActive
}

func (e *entry) setClosed() {
	e.mx.Lock()
	defer e.mx.Unlock()
	close(e.close)
	e.state = entryStateClosed
}

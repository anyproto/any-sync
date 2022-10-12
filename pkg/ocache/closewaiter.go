package ocache

import (
	"context"
	"sync"
)

type CloseWrapper struct {
	Value Object
	ch    chan struct{}
}

func (c *CloseWrapper) Close() (err error) {
	err = c.Value.Close()
	close(c.ch)
	return err
}

type CloseWaiter struct {
	load func(ctx context.Context, id string) (value Object, err error)

	mx       sync.Mutex
	closeMap map[string]chan struct{}
}

func NewCloseWaiter(load func(ctx context.Context, id string) (value Object, err error)) *CloseWaiter {
	return &CloseWaiter{
		load:     load,
		closeMap: make(map[string]chan struct{}),
	}
}

func (l *CloseWaiter) Load(ctx context.Context, id string) (value Object, err error) {
	// this uses assumption of ocache, that for each id load function cannot be called simultaneously
	var ch chan struct{}
	l.mx.Lock()
	if c, exists := l.closeMap[id]; exists {
		ch = c
	}
	l.mx.Unlock()
	if ch != nil {
		<-ch
	}

	value, err = l.load(ctx, id)
	if err != nil {
		return
	}

	ch = make(chan struct{})
	l.mx.Lock()
	l.closeMap[id] = ch
	l.mx.Unlock()

	value = &CloseWrapper{
		Value: value,
		ch:    ch,
	}
	return
}

package multiqueue

import (
	"errors"
	"sync"
)

type QueueProvider[T any] interface {
	GetQueue(id string) *Queue[T]
	RemoveQueue(id string) error
	Close() error
}

type queueProvider[T any] struct {
	queues  map[string]*Queue[T]
	mx      sync.Mutex
	closed  bool
	size    int
	handler QueueHandler[T]
}

func NewQueueProvider[T any](size int, handler QueueHandler[T]) QueueProvider[T] {
	return &queueProvider[T]{
		queues:  make(map[string]*Queue[T]),
		size:    size,
		handler: handler,
	}
}

func (p *queueProvider[T]) GetQueue(id string) *Queue[T] {
	p.mx.Lock()
	defer p.mx.Unlock()
	if p.closed {
		return nil
	}
	q, ok := p.queues[id]
	if !ok {
		q = NewQueue(id, p.size, p.handler)
		p.queues[id] = q
	}
	return q
}

func (p *queueProvider[T]) RemoveQueue(id string) error {
	p.mx.Lock()
	defer p.mx.Unlock()
	if p.closed {
		return nil
	}
	q, ok := p.queues[id]
	if !ok {
		return nil
	}
	delete(p.queues, id)
	return q.Close()
}

func (p *queueProvider[T]) Close() error {
	p.mx.Lock()
	defer p.mx.Unlock()
	p.closed = true
	var err error
	for _, q := range p.queues {
		qErr := q.Close()
		if qErr != nil {
			err = errors.Join(err, qErr)
		}
	}
	return err
}

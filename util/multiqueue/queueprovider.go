package multiqueue

import (
	"sync"
)

type QueueProvider[T any] interface {
	GetQueue(id string) *Queue[T]
	RemoveQueue(id string) error
}

type queueProvider[T any] struct {
	queues  map[string]*Queue[T]
	mx      sync.Mutex
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
	q, ok := p.queues[id]
	if !ok {
		return nil
	}
	delete(p.queues, id)
	return q.Close()
}

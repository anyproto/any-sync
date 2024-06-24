package multiqueue

import (
	"errors"
	"sync"
)

type QueueProvider[T Sizeable] interface {
	GetQueue(id string) *Queue[T]
	RemoveQueue(id string) error
	Close() error
}

type queueProvider[T Sizeable] struct {
	queues  map[string]*Queue[T]
	mx      sync.Mutex
	closed  bool
	size    int
	msgType int
	handler QueueHandler[T]
	updater sizeUpdater
}

func NewQueueProvider[T Sizeable](size, msgType int, updater sizeUpdater, handler QueueHandler[T]) QueueProvider[T] {
	return &queueProvider[T]{
		queues:  make(map[string]*Queue[T]),
		updater: updater,
		size:    size,
		msgType: msgType,
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
		q = NewQueue(id, p.size, p.msgType, p.updater, p.handler)
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

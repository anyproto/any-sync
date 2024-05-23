package multiqueue

import (
	"context"

	"github.com/cheggaaa/mb/v3"
)

type QueueHandler[T any] func(id string, msg T, q *mb.MB[T]) error

type Queue[T any] struct {
	id      string
	q       *mb.MB[T]
	handler QueueHandler[T]
}

func NewQueue[T any](id string, size int, h QueueHandler[T]) *Queue[T] {
	return &Queue[T]{
		id:      id,
		q:       mb.New[T](size),
		handler: h,
	}
}

func (q *Queue[T]) TryAdd(msg T) error {
	return q.handler(q.id, msg, q.q)
}

func (q *Queue[T]) WaitOne(ctx context.Context) (T, error) {
	return q.q.WaitOne(ctx)
}

func (q *Queue[T]) Close() error {
	return q.q.Close()
}

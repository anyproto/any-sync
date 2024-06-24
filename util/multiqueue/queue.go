package multiqueue

import (
	"context"

	"github.com/cheggaaa/mb/v3"
)

type QueueHandler[T Sizeable] func(id string, msg T, q *mb.MB[T]) error

type Queue[T Sizeable] struct {
	id      string
	q       *mb.MB[T]
	msgType int
	updater sizeUpdater
	handler QueueHandler[T]
}

func NewQueue[T Sizeable](id string, size, msgType int, updater sizeUpdater, h QueueHandler[T]) *Queue[T] {
	return &Queue[T]{
		id:      id,
		updater: updater,
		msgType: msgType,
		q:       mb.New[T](size),
		handler: h,
	}
}

func (q *Queue[T]) TryAdd(msg T) error {
	err := q.handler(q.id, msg, q.q)
	if err != nil {
		return err
	}
	q.updateSize(msg, true)
	return nil
}

func (q *Queue[T]) WaitOne(ctx context.Context) (T, error) {
	res, err := q.q.WaitOne(ctx)
	if err != nil {
		return res, err
	}
	q.updateSize(res, false)
	return res, nil
}

func (q *Queue[T]) Close() error {
	return q.q.Close()
}

func (q *Queue[T]) updateSize(msg T, add bool) {
	q.updater.UpdateQueueSize(msg.MsgSize(), q.msgType, add)
}

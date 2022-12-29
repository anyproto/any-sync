package objectsync

import (
	"context"
	"github.com/cheggaaa/mb/v3"
	"go.uber.org/zap"
)

type ActionFunc func() error

type ActionQueue interface {
	Send(action ActionFunc) (err error)
	Run()
	Close()
}

type actionQueue struct {
	batcher     *mb.MB[ActionFunc]
	maxReaders  int
	maxQueueLen int
	readers     chan struct{}
}

func NewActionQueue(maxReaders int, maxQueueLen int) ActionQueue {
	return &actionQueue{
		batcher:     mb.New[ActionFunc](maxQueueLen),
		maxReaders:  maxReaders,
		maxQueueLen: maxQueueLen,
	}
}

func (q *actionQueue) Send(action ActionFunc) (err error) {
	log.Debug("adding action to batcher")
	err = q.batcher.TryAdd(action)
	if err == nil {
		return
	}
	log.With(zap.Error(err)).Debug("queue returned error")
	actions := q.batcher.GetAll()
	actions = actions[len(actions)/2:]
	return q.batcher.Add(context.Background(), actions...)
}

func (q *actionQueue) Run() {
	log.Debug("running the queue")
	q.readers = make(chan struct{}, q.maxReaders)
	for i := 0; i < q.maxReaders; i++ {
		go q.startReading()
	}
}

func (q *actionQueue) startReading() {
	defer func() {
		q.readers <- struct{}{}
	}()
	for {
		action, err := q.batcher.WaitOne(context.Background())
		if err != nil {
			return
		}
		err = action()
		if err != nil {
			log.With(zap.Error(err)).Debug("action errored out")
		}
	}
}

func (q *actionQueue) Close() {
	q.batcher.Close()
	for i := 0; i < q.maxReaders; i++ {
		<-q.readers
	}
}

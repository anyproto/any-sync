package syncservice

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
	batcher   *mb.MB[ActionFunc]
	ctx       context.Context
	cancel    context.CancelFunc
	queueDone chan struct{}
}

func NewActionQueue() ActionQueue {
	return &actionQueue{
		batcher:   mb.New[ActionFunc](0),
		ctx:       nil,
		cancel:    nil,
		queueDone: make(chan struct{}),
	}
}

func (q *actionQueue) Send(action ActionFunc) (err error) {
	log.Debug("adding action to batcher")
	return q.batcher.Add(q.ctx, action)
}

func (q *actionQueue) Run() {
	log.Debug("running the queue")
	q.ctx, q.cancel = context.WithCancel(context.Background())
	go q.read()
}

func (q *actionQueue) read() {
	limiter := make(chan struct{}, maxSimultaneousOperationsPerStream)
	for i := 0; i < maxSimultaneousOperationsPerStream; i++ {
		limiter <- struct{}{}
	}
	defer func() {
		// wait until all operations are done
		for i := 0; i < maxSimultaneousOperationsPerStream; i++ {
			<-limiter
		}
		close(q.queueDone)
	}()
	doSendActions := func() {
		actions, err := q.batcher.Wait(q.ctx)
		log.Debug("reading from batcher")
		if err != nil {
			log.With(zap.Error(err)).Error("queue finished")
			return
		}
		for _, msg := range actions {
			<-limiter
			go func(action ActionFunc) {
				err = action()
				if err != nil {
					log.With(zap.Error(err)).Debug("action errored out")
				}
				limiter <- struct{}{}
			}(msg)
		}
	}
	for {
		select {
		case <-q.ctx.Done():
			return
		default:
			doSendActions()
		}
	}
}

func (q *actionQueue) Close() {
	q.cancel()
	<-q.queueDone
}

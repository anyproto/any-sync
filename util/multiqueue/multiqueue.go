package multiqueue

import (
	"context"
	"errors"
	"github.com/cheggaaa/mb/v3"
	"sync"
)

var (
	ErrThreadNotExists = errors.New("multiQueue: thread not exists")
	ErrClosed          = errors.New("multiQueue: closed")
)

func New[T any](h HandleFunc[T], maxThreadSize int) MultiQueue[T] {
	return &multiQueue[T]{
		handler:      h,
		threads:      make(map[string]*mb.MB[T]),
		queueMaxSize: maxThreadSize,
	}
}

type HandleFunc[T any] func(msg T)

type MultiQueue[T any] interface {
	Add(ctx context.Context, threadId string, msg T) (err error)
	CloseThread(threadId string) (err error)
	Close() (err error)
}

type multiQueue[T any] struct {
	handler      HandleFunc[T]
	queueMaxSize int
	threads      map[string]*mb.MB[T]
	mu           sync.Mutex
	closed       bool
}

func (m *multiQueue[T]) Add(ctx context.Context, threadId string, msg T) (err error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return ErrClosed
	}
	q, ok := m.threads[threadId]
	if !ok {
		q = m.startThread(threadId)
	}
	m.mu.Unlock()
	return q.TryAdd(msg)
}

func (m *multiQueue[T]) startThread(id string) *mb.MB[T] {
	q := mb.New[T](m.queueMaxSize)
	m.threads[id] = q
	go m.threadLoop(q)
	return q
}

func (m *multiQueue[T]) threadLoop(q *mb.MB[T]) {
	for {
		msg, err := q.WaitOne(context.Background())
		if err != nil {
			return
		}
		m.handler(msg)
	}
}

func (m *multiQueue[T]) CloseThread(threadId string) (err error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return ErrClosed
	}
	q, ok := m.threads[threadId]
	if ok {
		delete(m.threads, threadId)
	}
	m.mu.Unlock()
	if !ok {
		return ErrThreadNotExists
	}
	return q.Close()
}

func (m *multiQueue[T]) Close() (err error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return ErrClosed
	}
	m.closed = true
	threads := m.threads
	m.mu.Unlock()
	for _, q := range threads {
		_ = q.Close()
	}
	return nil
}

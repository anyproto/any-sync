package multiqueue

import (
	"context"
	"errors"
	"sync"

	"github.com/cheggaaa/mb/v3"
)

var (
	ErrThreadNotExists = errors.New("multiQueue: thread not exists")
	ErrClosed          = errors.New("multiQueue: closed")
)

func New[T Sizeable](h HandleFunc[T], updater sizeUpdater, msgType int, maxThreadSize int) MultiQueue[T] {
	return &multiQueue[T]{
		handler:      h,
		updater:      updater,
		msgType:      msgType,
		threads:      make(map[string]*mb.MB[T]),
		queueMaxSize: maxThreadSize,
	}
}

type HandleFunc[T any] func(msg T)

type Sizeable interface {
	MsgSize() uint64
}

type sizeUpdater interface {
	UpdateQueueSize(size uint64, msgType int, add bool)
}

type MultiQueue[T Sizeable] interface {
	Add(ctx context.Context, threadId string, msg T) (err error)
	CloseThread(threadId string) (err error)
	ThreadIds() []string
	Close() (err error)
}

type multiQueue[T Sizeable] struct {
	handler      HandleFunc[T]
	updater      sizeUpdater
	queueMaxSize int
	msgType      int
	threads      map[string]*mb.MB[T]
	mu           sync.Mutex
	closed       bool
}

func (m *multiQueue[T]) ThreadIds() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]string, 0, len(m.threads))
	for id := range m.threads {
		ids = append(ids, id)
	}
	return ids
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
	m.updateSize(msg, true)
	err = q.TryAdd(msg)
	if err != nil {
		m.updateSize(msg, false)
		return
	}
	return
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
		m.updateSize(msg, false)
		m.handler(msg)
	}
}

func (m *multiQueue[T]) updateSize(msg T, add bool) {
	m.updater.UpdateQueueSize(msg.MsgSize(), m.msgType, add)
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

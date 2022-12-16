package synctree

import (
	"errors"
	"sync"
)

type ReceiveQueue interface {
	AddMessage(senderId string, msg treeMsg) (queueFull bool)
	GetMessage(senderId string) (msg treeMsg, err error)
	ClearQueue(senderId string)
}

type receiveQueue struct {
	sync.Mutex
	handlerMap map[string][]treeMsg
	maxSize    int
}

func newReceiveQueue(maxSize int) ReceiveQueue {
	return &receiveQueue{
		Mutex:      sync.Mutex{},
		handlerMap: map[string][]treeMsg{},
		maxSize:    maxSize,
	}
}

var errEmptyQueue = errors.New("the queue is empty")

func (q *receiveQueue) AddMessage(senderId string, msg treeMsg) (queueFull bool) {
	q.Lock()
	defer q.Unlock()

	queue := q.handlerMap[senderId]
	queueFull = len(queue) >= maxQueueSize
	queue = append(queue, msg)
	q.handlerMap[senderId] = queue

	return
}

func (q *receiveQueue) GetMessage(senderId string) (msg treeMsg, err error) {
	q.Lock()
	defer q.Unlock()

	if len(q.handlerMap) == 0 {
		err = errEmptyQueue
		return
	}

	msg = q.handlerMap[senderId][0]
	return
}

func (q *receiveQueue) ClearQueue(senderId string) {
	q.Lock()
	defer q.Unlock()

	queue := q.handlerMap[senderId]
	excessLen := len(queue) - q.maxSize + 1
	if excessLen <= 0 {
		excessLen = 1
	}
	queue = queue[excessLen:]
	q.handlerMap[senderId] = queue
}

package synctree

import (
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/treechangeproto"
	"sync"
)

type ReceiveQueue interface {
	AddMessage(senderId string, msg *treechangeproto.TreeSyncMessage, replyId string) (queueFull bool)
	GetMessage(senderId string) (msg *treechangeproto.TreeSyncMessage, replyId string, err error)
	ClearQueue(senderId string)
}

type queueMsg struct {
	replyId     string
	syncMessage *treechangeproto.TreeSyncMessage
}

type receiveQueue struct {
	sync.Mutex
	handlerMap map[string][]queueMsg
	maxSize    int
}

func newReceiveQueue(maxSize int) ReceiveQueue {
	return &receiveQueue{
		Mutex:      sync.Mutex{},
		handlerMap: map[string][]queueMsg{},
		maxSize:    maxSize,
	}
}

var errEmptyQueue = errors.New("the queue is empty")

func (q *receiveQueue) AddMessage(senderId string, msg *treechangeproto.TreeSyncMessage, replyId string) (queueFull bool) {
	q.Lock()
	defer q.Unlock()

	queue := q.handlerMap[senderId]
	queueFull = len(queue) >= maxQueueSize
	queue = append(queue, queueMsg{replyId, msg})
	q.handlerMap[senderId] = queue

	return
}

func (q *receiveQueue) GetMessage(senderId string) (msg *treechangeproto.TreeSyncMessage, replyId string, err error) {
	q.Lock()
	defer q.Unlock()

	if len(q.handlerMap) == 0 {
		err = errEmptyQueue
		return
	}

	qMsg := q.handlerMap[senderId][0]
	msg = qMsg.syncMessage
	replyId = qMsg.replyId
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

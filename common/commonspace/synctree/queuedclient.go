package synctree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
)

type queuedClient struct {
	SyncClient
	queue syncservice.ActionQueue
}

func newQueuedClient(client SyncClient, queue syncservice.ActionQueue) SyncClient {
	return &queuedClient{
		SyncClient: client,
		queue:      queue,
	}
}

func (q *queuedClient) BroadcastAsync(message *treechangeproto.TreeSyncMessage) (err error) {
	return q.queue.Send(func() error {
		return q.SyncClient.BroadcastAsync(message)
	})
}

func (q *queuedClient) SendAsync(peerId string, message *treechangeproto.TreeSyncMessage, replyId string) (err error) {
	return q.queue.Send(func() error {
		return q.SyncClient.SendAsync(peerId, message, replyId)
	})
}

func (q *queuedClient) BroadcastAsyncOrSendResponsible(message *treechangeproto.TreeSyncMessage) (err error) {
	return q.queue.Send(func() error {
		return q.SyncClient.BroadcastAsyncOrSendResponsible(message)
	})
}

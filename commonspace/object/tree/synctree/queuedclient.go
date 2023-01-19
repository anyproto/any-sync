package synctree

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/objectsync"
)

type queuedClient struct {
	SyncClient
	queue objectsync.ActionQueue
}

func newQueuedClient(client SyncClient, queue objectsync.ActionQueue) SyncClient {
	return &queuedClient{
		SyncClient: client,
		queue:      queue,
	}
}

func (q *queuedClient) Broadcast(ctx context.Context, message *treechangeproto.TreeSyncMessage) (err error) {
	return q.queue.Send(func() error {
		return q.SyncClient.Broadcast(ctx, message)
	})
}

func (q *queuedClient) SendWithReply(ctx context.Context, peerId string, message *treechangeproto.TreeSyncMessage, replyId string) (err error) {
	return q.queue.Send(func() error {
		return q.SyncClient.SendWithReply(ctx, peerId, message, replyId)
	})
}

func (q *queuedClient) BroadcastAsyncOrSendResponsible(ctx context.Context, message *treechangeproto.TreeSyncMessage) (err error) {
	return q.queue.Send(func() error {
		return q.SyncClient.BroadcastAsyncOrSendResponsible(ctx, message)
	})
}

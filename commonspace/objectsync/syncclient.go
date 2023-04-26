package objectsync

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"go.uber.org/zap"
)

type SyncClient interface {
	RequestFactory
	Broadcast(ctx context.Context, msg *treechangeproto.TreeSyncMessage)
	SendWithReply(ctx context.Context, peerId, objectId string, msg *treechangeproto.TreeSyncMessage, replyId string) (err error)
	SendSync(ctx context.Context, peerId, objectId string, msg *treechangeproto.TreeSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error)
	MessagePool() MessagePool
}

type syncClient struct {
	RequestFactory
	spaceId     string
	messagePool MessagePool
}

func NewSyncClient(
	spaceId string,
	messagePool MessagePool,
	factory RequestFactory) SyncClient {
	return &syncClient{
		messagePool:    messagePool,
		RequestFactory: factory,
		spaceId:        spaceId,
	}
}

func (s *syncClient) Broadcast(ctx context.Context, msg *treechangeproto.TreeSyncMessage) {
	objMsg, err := MarshallTreeMessage(msg, s.spaceId, msg.RootChange.Id, "")
	if err != nil {
		return
	}
	err = s.messagePool.Broadcast(ctx, objMsg)
	if err != nil {
		log.DebugCtx(ctx, "broadcast error", zap.Error(err))
	}
}

func (s *syncClient) SendSync(ctx context.Context, peerId, objectId string, msg *treechangeproto.TreeSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error) {
	objMsg, err := MarshallTreeMessage(msg, s.spaceId, objectId, "")
	if err != nil {
		return
	}
	return s.messagePool.SendSync(ctx, peerId, objMsg)
}

func (s *syncClient) SendWithReply(ctx context.Context, peerId, objectId string, msg *treechangeproto.TreeSyncMessage, replyId string) (err error) {
	objMsg, err := MarshallTreeMessage(msg, s.spaceId, objectId, replyId)
	if err != nil {
		return
	}
	return s.messagePool.SendPeer(ctx, peerId, objMsg)
}

func (s *syncClient) MessagePool() MessagePool {
	return s.messagePool
}

func MarshallTreeMessage(message *treechangeproto.TreeSyncMessage, spaceId, objectId, replyId string) (objMsg *spacesyncproto.ObjectSyncMessage, err error) {
	payload, err := message.Marshal()
	if err != nil {
		return
	}
	objMsg = &spacesyncproto.ObjectSyncMessage{
		ReplyId:  replyId,
		Payload:  payload,
		ObjectId: objectId,
		SpaceId:  spaceId,
	}
	return
}

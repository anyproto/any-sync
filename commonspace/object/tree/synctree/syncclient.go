//go:generate mockgen -destination mock_synctree/mock_synctree.go github.com/anytypeio/any-sync/commonspace/object/tree/synctree SyncClient,SyncTree,ReceiveQueue,HeadNotifiable
package synctree

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/confconnector"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/objectsync"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/nodeconf"
)

type SyncClient interface {
	RequestFactory
	Broadcast(ctx context.Context, msg *treechangeproto.TreeSyncMessage) (err error)
	BroadcastAsyncOrSendResponsible(ctx context.Context, msg *treechangeproto.TreeSyncMessage) (err error)
	SendWithReply(ctx context.Context, peerId string, msg *treechangeproto.TreeSyncMessage, replyId string) (err error)
}

type syncClient struct {
	objectsync.MessagePool
	RequestFactory
	spaceId       string
	connector     confconnector.ConfConnector
	configuration nodeconf.Configuration
}

func newSyncClient(
	spaceId string,
	pool objectsync.MessagePool,
	factory RequestFactory,
	configuration nodeconf.Configuration) SyncClient {
	return &syncClient{
		MessagePool:    pool,
		RequestFactory: factory,
		configuration:  configuration,
		spaceId:        spaceId,
	}
}

func (s *syncClient) Broadcast(ctx context.Context, msg *treechangeproto.TreeSyncMessage) (err error) {
	objMsg, err := marshallTreeMessage(msg, s.spaceId, msg.RootChange.Id, "")
	if err != nil {
		return
	}
	return s.MessagePool.Broadcast(ctx, objMsg)
}

func (s *syncClient) SendWithReply(ctx context.Context, peerId string, msg *treechangeproto.TreeSyncMessage, replyId string) (err error) {
	objMsg, err := marshallTreeMessage(msg, s.spaceId, msg.RootChange.Id, replyId)
	if err != nil {
		return
	}
	return s.MessagePool.SendPeer(ctx, peerId, objMsg)
}

func (s *syncClient) BroadcastAsyncOrSendResponsible(ctx context.Context, message *treechangeproto.TreeSyncMessage) (err error) {
	objMsg, err := marshallTreeMessage(message, s.spaceId, message.RootChange.Id, "")
	if err != nil {
		return
	}

	if s.configuration.IsResponsible(s.spaceId) {
		return s.MessagePool.SendResponsible(ctx, objMsg)
	}
	return s.Broadcast(ctx, message)
}

func marshallTreeMessage(message *treechangeproto.TreeSyncMessage, spaceId, objectId, replyId string) (objMsg *spacesyncproto.ObjectSyncMessage, err error) {
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

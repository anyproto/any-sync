//go:generate mockgen -destination mock_synctree/mock_synctree.go github.com/anytypeio/any-sync/commonspace/object/tree/synctree SyncClient,SyncTree,ReceiveQueue,HeadNotifiable
package synctree

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/peermanager"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
)

type SyncClient interface {
	RequestFactory
	Broadcast(ctx context.Context, msg *treechangeproto.TreeSyncMessage) (err error)
	SendWithReply(ctx context.Context, peerId string, msg *treechangeproto.TreeSyncMessage, replyId string) (err error)
}

type syncClient struct {
	peermanager.PeerManager
	RequestFactory
	spaceId string
}

func NewSyncClient(
	spaceId string,
	peerManager peermanager.PeerManager,
	factory RequestFactory) SyncClient {
	return &syncClient{
		PeerManager:    peerManager,
		RequestFactory: factory,
		spaceId:        spaceId,
	}
}

func (s *syncClient) Broadcast(ctx context.Context, msg *treechangeproto.TreeSyncMessage) (err error) {
	objMsg, err := marshallTreeMessage(msg, s.spaceId, msg.RootChange.Id, "")
	if err != nil {
		return
	}
	return s.PeerManager.Broadcast(ctx, objMsg)
}

func (s *syncClient) SendWithReply(ctx context.Context, peerId string, msg *treechangeproto.TreeSyncMessage, replyId string) (err error) {
	objMsg, err := marshallTreeMessage(msg, s.spaceId, msg.RootChange.Id, replyId)
	if err != nil {
		return
	}
	return s.PeerManager.SendPeer(ctx, peerId, objMsg)
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

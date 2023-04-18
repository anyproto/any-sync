package objectsync

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
	PeerManager() peermanager.PeerManager
}

type syncClient struct {
	RequestFactory
	spaceId     string
	peerManager peermanager.PeerManager
}

func NewSyncClient(
	spaceId string,
	peerManager peermanager.PeerManager,
	factory RequestFactory) SyncClient {
	return &syncClient{
		peerManager:    peerManager,
		RequestFactory: factory,
		spaceId:        spaceId,
	}
}

func (s *syncClient) Broadcast(ctx context.Context, msg *treechangeproto.TreeSyncMessage) (err error) {
	objMsg, err := MarshallTreeMessage(msg, s.spaceId, msg.RootChange.Id, "")
	if err != nil {
		return
	}
	return s.peerManager.Broadcast(ctx, objMsg)
}

func (s *syncClient) SendWithReply(ctx context.Context, peerId string, msg *treechangeproto.TreeSyncMessage, replyId string) (err error) {
	objMsg, err := MarshallTreeMessage(msg, s.spaceId, msg.RootChange.Id, replyId)
	if err != nil {
		return
	}
	return s.peerManager.SendPeer(ctx, peerId, objMsg)
}

func (s *syncClient) PeerManager() peermanager.PeerManager {
	return s.peerManager
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

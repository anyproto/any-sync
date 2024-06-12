package synctree

import (
	"context"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/peermanager"
	"github.com/anyproto/any-sync/commonspace/requestmanager"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type SyncClient interface {
	RequestFactory
	Broadcast(msg *treechangeproto.TreeSyncMessage)
	SendUpdate(peerId, objectId string, msg *treechangeproto.TreeSyncMessage) (err error)
	QueueRequest(peerId, objectId string, msg *treechangeproto.TreeSyncMessage) (err error)
	SendRequest(ctx context.Context, peerId, objectId string, msg *treechangeproto.TreeSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error)
}

type syncClient struct {
	RequestFactory
	spaceId string
}

func NewSyncClient(spaceId string, requestManager requestmanager.RequestManager, peerManager peermanager.PeerManager) SyncClient {
	return &syncClient{
		RequestFactory: &requestFactory{},
		spaceId:        spaceId,
		requestManager: requestManager,
		peerManager:    peerManager,
	}
}

func (s *syncClient) Broadcast(msg *treechangeproto.TreeSyncMessage) {
	objMsg, err := spacesyncproto.MarshallSyncMessage(msg, s.spaceId, msg.RootChange.Id)
	if err != nil {
		return
	}
	err = s.peerManager.Broadcast(context.Background(), objMsg)
	if err != nil {
		log.Debug("broadcast error", zap.Error(err))
	}
}

func (s *syncClient) SendUpdate(peerId, objectId string, msg *treechangeproto.TreeSyncMessage) (err error) {
	objMsg, err := spacesyncproto.MarshallSyncMessage(msg, s.spaceId, objectId)
	if err != nil {
		return
	}
	return s.peerManager.SendPeer(context.Background(), peerId, objMsg)
}

func (s *syncClient) SendRequest(ctx context.Context, peerId, objectId string, msg *treechangeproto.TreeSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error) {
	objMsg, err := spacesyncproto.MarshallSyncMessage(msg, s.spaceId, objectId)
	if err != nil {
		return
	}
	return s.requestManager.SendRequest(ctx, peerId, objMsg)
}

func (s *syncClient) QueueRequest(peerId, objectId string, msg *treechangeproto.TreeSyncMessage) (err error) {
	objMsg, err := spacesyncproto.MarshallSyncMessage(msg, s.spaceId, objectId)
	if err != nil {
		return
	}
	return s.requestManager.QueueRequest(peerId, objMsg)
}

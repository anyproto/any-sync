package syncacl

import (
	"context"

	"github.com/anyproto/any-sync/commonspace/peermanager"
	"github.com/anyproto/any-sync/commonspace/requestmanager"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"go.uber.org/zap"
)

type SyncClient interface {
	RequestFactory
	Broadcast(msg *consensusproto.LogSyncMessage)
	SendUpdate(peerId string, msg *consensusproto.LogSyncMessage) (err error)
	QueueRequest(peerId string, msg *consensusproto.LogSyncMessage) (err error)
	SendRequest(ctx context.Context, peerId string, msg *consensusproto.LogSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error)
}

type syncClient struct {
	RequestFactory
	spaceId        string
	requestManager requestmanager.RequestManager
	peerManager    peermanager.PeerManager
}

func NewSyncClient(spaceId string, requestManager requestmanager.RequestManager, peerManager peermanager.PeerManager) SyncClient {
	return &syncClient{
		RequestFactory: &requestFactory{},
		spaceId:        spaceId,
		requestManager: requestManager,
		peerManager:    peerManager,
	}
}

func (s *syncClient) Broadcast(msg *consensusproto.LogSyncMessage) {
	objMsg, err := spacesyncproto.MarshallSyncMessage(msg, s.spaceId, msg.Id)
	if err != nil {
		return
	}
	err = s.peerManager.Broadcast(context.Background(), objMsg)
	if err != nil {
		log.Debug("broadcast error", zap.Error(err))
	}
}

func (s *syncClient) SendUpdate(peerId string, msg *consensusproto.LogSyncMessage) (err error) {
	objMsg, err := spacesyncproto.MarshallSyncMessage(msg, s.spaceId, msg.Id)
	if err != nil {
		return
	}
	return s.peerManager.SendPeer(context.Background(), peerId, objMsg)
}

func (s *syncClient) SendRequest(ctx context.Context, peerId string, msg *consensusproto.LogSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error) {
	objMsg, err := spacesyncproto.MarshallSyncMessage(msg, s.spaceId, msg.Id)
	if err != nil {
		return
	}
	return s.requestManager.SendRequest(ctx, peerId, objMsg)
}

func (s *syncClient) QueueRequest(peerId string, msg *consensusproto.LogSyncMessage) (err error) {
	objMsg, err := spacesyncproto.MarshallSyncMessage(msg, s.spaceId, msg.Id)
	if err != nil {
		return
	}
	return s.requestManager.QueueRequest(peerId, objMsg)
}

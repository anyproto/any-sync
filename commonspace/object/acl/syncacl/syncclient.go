package syncacl

import (
	"context"

	"github.com/anyproto/any-sync/commonspace/peermanager"
	"github.com/anyproto/any-sync/commonspace/requestmanager"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

type SyncClient interface {
	RequestFactory
	Broadcast(msg *consensusproto.LogSyncMessage)
	SendUpdate(peerId, objectId string, msg *consensusproto.LogSyncMessage) (err error)
	QueueRequest(peerId, objectId string, msg *consensusproto.LogSyncMessage) (err error)
	SendRequest(ctx context.Context, peerId, objectId string, msg *consensusproto.LogSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error)
}

type syncClient struct {
	RequestFactory
	spaceId        string
	requestManager requestmanager.RequestManager
	peerManager    peermanager.PeerManager
}

func (s *syncClient) Broadcast(msg *consensusproto.LogSyncMessage) {
}

func (s *syncClient) SendUpdate(peerId, objectId string, msg *consensusproto.LogSyncMessage) (err error) {
	return
}

func (s *syncClient) QueueRequest(peerId, objectId string, msg *consensusproto.LogSyncMessage) (err error) {
	return
}

func (s *syncClient) SendRequest(ctx context.Context, peerId, objectId string, msg *consensusproto.LogSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error) {
	return
}

func NewSyncClient(spaceId string, requestManager requestmanager.RequestManager, peerManager peermanager.PeerManager) SyncClient {
	return &syncClient{
		RequestFactory: &requestFactory{},
		spaceId:        spaceId,
		requestManager: requestManager,
		peerManager:    peerManager,
	}
}

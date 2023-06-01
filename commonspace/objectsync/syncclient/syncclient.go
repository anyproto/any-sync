package syncclient

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/requestsender"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/streamsender"
	"go.uber.org/zap"
)

const CName = "common.objectsync.syncclient"

var log = logger.NewNamed(CName)

type SyncClient interface {
	app.Component
	RequestFactory
	Broadcast(msg *treechangeproto.TreeSyncMessage)
	SendUpdate(peerId, objectId string, msg *treechangeproto.TreeSyncMessage) (err error)
	QueueRequest(peerId, objectId string, msg *treechangeproto.TreeSyncMessage) (err error)
	SendRequest(ctx context.Context, peerId, objectId string, msg *treechangeproto.TreeSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error)
}

type syncClient struct {
	RequestFactory
	spaceId       string
	requestSender requestsender.RequestSender
	streamSender  streamsender.StreamSender
}

func New() SyncClient {
	return &syncClient{
		RequestFactory: &requestFactory{},
	}
}

func (s *syncClient) Init(a *app.App) (err error) {
	sharedState := a.MustComponent(spacestate.CName).(*spacestate.SpaceState)
	s.spaceId = sharedState.SpaceId
	s.requestSender = a.MustComponent(requestsender.CName).(requestsender.RequestSender)
	s.streamSender = a.MustComponent(streamsender.CName).(streamsender.StreamSender)
	return nil
}

func (s *syncClient) Name() (name string) {
	return CName
}

func (s *syncClient) Broadcast(msg *treechangeproto.TreeSyncMessage) {
	objMsg, err := MarshallTreeMessage(msg, s.spaceId, msg.RootChange.Id, "")
	if err != nil {
		return
	}
	err = s.streamSender.Broadcast(objMsg)
	if err != nil {
		log.Debug("broadcast error", zap.Error(err))
	}
}

func (s *syncClient) SendUpdate(peerId, objectId string, msg *treechangeproto.TreeSyncMessage) (err error) {
	objMsg, err := MarshallTreeMessage(msg, s.spaceId, objectId, "")
	if err != nil {
		return
	}
	return s.streamSender.SendPeer(peerId, objMsg)
}

func (s *syncClient) SendRequest(ctx context.Context, peerId, objectId string, msg *treechangeproto.TreeSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error) {
	objMsg, err := MarshallTreeMessage(msg, s.spaceId, objectId, "")
	if err != nil {
		return
	}
	return s.requestSender.SendRequest(ctx, peerId, objMsg)
}

func (s *syncClient) QueueRequest(peerId, objectId string, msg *treechangeproto.TreeSyncMessage) (err error) {
	objMsg, err := MarshallTreeMessage(msg, s.spaceId, objectId, "")
	if err != nil {
		return
	}
	return s.requestSender.QueueRequest(peerId, objMsg)
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

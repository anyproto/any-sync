package message

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/node"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/sync/requesthandler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"sync"
	"time"
)

var log = logger.NewNamed("messageservice")

const CName = "MessageService"

type service struct {
	nodes          []*node.Node
	requestHandler requesthandler.RequestHandler
	pool           pool.Pool
	sync.RWMutex
}

func New() app.Component {
	return &service{}
}

type Service interface {
	SendMessage(ctx context.Context, peerId string, msg *syncproto.Sync) error
	SendToSpace(ctx context.Context, spaceId string, msg *syncproto.Sync) error
}

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	s.requestHandler = a.MustComponent(requesthandler.CName).(requesthandler.RequestHandler)
	s.nodes = a.MustComponent(node.CName).(node.Service).Nodes()
	s.pool = a.MustComponent(pool.CName).(pool.Pool)
	s.pool.AddHandler(syncproto.MessageType_MessageTypeSync, s.HandleMessage)
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	return nil
}

func (s *service) Close(ctx context.Context) (err error) {
	return nil
}

func (s *service) HandleMessage(ctx context.Context, msg *pool.Message) (err error) {
	log.With(
		zap.String("peerId", msg.Peer().Id())).
		Debug("handling message from peer")

	err = msg.Ack()
	if err != nil {
		log.With(zap.String("peerId", msg.Peer().Id()), zap.Error(err)).
			Error("could not ack message")
	} else {
		log.With(zap.String("peerId", msg.Peer().Id()), zap.Int("type", int(msg.Header.Type))).
			Debug("ack returned")
	}
	syncMsg := &syncproto.Sync{}
	err = proto.Unmarshal(msg.Data, syncMsg)
	if err != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	return s.requestHandler.HandleSyncMessage(timeoutCtx, msg.Peer().Id(), syncMsg)
}

func (s *service) SendMessage(ctx context.Context, peerId string, msg *syncproto.Sync) error {
	log.With(
		zap.String("peerId", peerId),
		zap.String("message", msgType(msg))).
		Debug("sending message to peer")

	err := s.pool.DialAndAddPeer(context.Background(), peerId)
	if err != nil {
		return err
	}

	marshalled, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	err = s.pool.SendAndWait(ctx, peerId, &syncproto.Message{
		Header: &syncproto.Header{Type: syncproto.MessageType_MessageTypeSync},
		Data:   marshalled,
	})

	if err != nil {
		log.With(
			zap.String("peerId", peerId),
			zap.String("message", msgType(msg)),
			zap.Error(err)).
			Debug("failed to send message to peer")
	} else {
		log.With(
			zap.String("peerId", peerId),
			zap.String("message", msgType(msg))).
			Debug("message send to peer completed")
	}
	return err
}

func (s *service) SendToSpace(ctx context.Context, spaceId string, msg *syncproto.Sync) error {
	for _, rp := range s.nodes {
		s.SendMessage(ctx, rp.PeerId, msg)
	}
	return nil
}

func msgType(content *syncproto.Sync) string {
	msg := content.GetMessage()
	switch {
	case msg.GetFullSyncRequest() != nil:
		return "FullSyncRequest"
	case msg.GetFullSyncResponse() != nil:
		return "FullSyncResponse"
	case msg.GetHeadUpdate() != nil:
		return "HeadUpdate"
	}
	return "UnknownMessage"
}

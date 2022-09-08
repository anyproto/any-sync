package message

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	pool2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/node"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/sync/requesthandler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
	"github.com/gogo/protobuf/proto"
	"sync"
	"time"
)

var log = logger.NewNamed("messageservice")

const CName = "MessageService"

type service struct {
	nodes          []*node.Node
	requestHandler requesthandler.RequestHandler
	pool           pool2.Pool
	sync.RWMutex
}

func New() app.Component {
	return &service{}
}

type Service interface {
	SendMessageAsync(peerId string, msg *syncproto.Sync) error
	SendToSpaceAsync(spaceId string, msg *syncproto.Sync) error
}

func (s *service) Init(a *app.App) (err error) {
	s.requestHandler = a.MustComponent(requesthandler.CName).(requesthandler.RequestHandler)
	s.nodes = a.MustComponent(node.CName).(node.Service).Nodes()
	s.pool = a.MustComponent(pool2.CName).(pool2.Pool)
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
	defer func() {
		if err != nil {
			msg.AckError(syncproto.System_Error_UNKNOWN, err.Error())
		} else {
			msg.Ack()
		}
	}()

	syncMsg := &syncproto.Sync{}
	err = proto.Unmarshal(msg.Data, syncMsg)
	if err != nil {
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	err = s.requestHandler.HandleSyncMessage(timeoutCtx, msg.Peer().Id(), syncMsg)
	return
}

func (s *service) SendMessageAsync(peerId string, msg *syncproto.Sync) (err error) {
	_, err = s.pool.DialAndAddPeer(context.Background(), peerId)
	if err != nil {
		return
	}

	marshalled, err := proto.Marshal(msg)
	if err != nil {
		return
	}

	go s.sendAsync(peerId, msgInfo(msg), marshalled)
	return
}

func (s *service) SendToSpaceAsync(spaceId string, msg *syncproto.Sync) error {
	for _, rp := range s.nodes {
		s.SendMessageAsync(rp.PeerId, msg)
	}
	return nil
}

func (s *service) sendAsync(peerId string, msgTypeStr string, marshalled []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	return s.pool.SendAndWait(ctx, peerId, &syncproto.Message{
		Header: &syncproto.Header{
			Type:      syncproto.MessageType_MessageTypeSync,
			DebugInfo: msgTypeStr,
		},
		Data: marshalled,
	})
}

func msgInfo(content *syncproto.Sync) (syncMethod string) {
	msg := content.GetMessage()
	switch {
	case msg.GetFullSyncRequest() != nil:
		syncMethod = "FullSyncRequest"
	case msg.GetFullSyncResponse() != nil:
		syncMethod = "FullSyncResponse"
	case msg.GetHeadUpdate() != nil:
		syncMethod = "HeadUpdate"
	}
	syncMethod = fmt.Sprintf("method: %s, treeType: %s", syncMethod, content.TreeHeader.DocType.String())
	return
}

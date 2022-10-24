//go:generate mockgen -destination mock_synctree/mock_synctree.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree SyncClient
package synctree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/diffservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
)

type SyncClient interface {
	RequestFactory
	BroadcastAsync(message *treechangeproto.TreeSyncMessage) (err error)
	BroadcastAsyncOrSendResponsible(message *treechangeproto.TreeSyncMessage) (err error)
	SendAsync(peerId string, message *treechangeproto.TreeSyncMessage, replyId string) (err error)
}

type syncClient struct {
	syncservice.StreamPool
	RequestFactory
	spaceId       string
	notifiable    diffservice.HeadNotifiable
	configuration nodeconf.Configuration
}

func newSyncClient(
	spaceId string,
	pool syncservice.StreamPool,
	notifiable diffservice.HeadNotifiable,
	factory RequestFactory,
	configuration nodeconf.Configuration) SyncClient {
	return &syncClient{
		StreamPool:     pool,
		RequestFactory: factory,
		notifiable:     notifiable,
		configuration:  configuration,
		spaceId:        spaceId,
	}
}

func (s *syncClient) BroadcastAsync(message *treechangeproto.TreeSyncMessage) (err error) {
	s.notifyIfNeeded(message)
	objMsg, err := marshallTreeMessage(message, message.RootChange.Id, "")
	if err != nil {
		return
	}
	return s.StreamPool.BroadcastAsync(objMsg)
}

func (s *syncClient) SendAsync(peerId string, message *treechangeproto.TreeSyncMessage, replyId string) (err error) {
	objMsg, err := marshallTreeMessage(message, message.RootChange.Id, replyId)
	if err != nil {
		return
	}
	return s.StreamPool.SendAsync([]string{peerId}, objMsg)
}

func (s *syncClient) BroadcastAsyncOrSendResponsible(message *treechangeproto.TreeSyncMessage) (err error) {
	s.notifyIfNeeded(message)
	objMsg, err := marshallTreeMessage(message, message.RootChange.Id, "")
	if err != nil {
		return
	}
	if s.configuration.IsResponsible(s.spaceId) {
		return s.StreamPool.SendAsync(s.configuration.NodeIds(s.spaceId), objMsg)
	}
	return s.BroadcastAsync(message)
}

func (s *syncClient) notifyIfNeeded(message *treechangeproto.TreeSyncMessage) {
	if message.GetContent().GetHeadUpdate() != nil {
		update := message.GetContent().GetHeadUpdate()
		s.notifiable.UpdateHeads(message.RootChange.Id, update.Heads)
	}
}

func marshallTreeMessage(message *treechangeproto.TreeSyncMessage, id, replyId string) (objMsg *spacesyncproto.ObjectSyncMessage, err error) {
	payload, err := message.Marshal()
	if err != nil {
		return
	}
	objMsg = &spacesyncproto.ObjectSyncMessage{
		ReplyId:  replyId,
		Payload:  payload,
		ObjectId: id,
	}
	return
}

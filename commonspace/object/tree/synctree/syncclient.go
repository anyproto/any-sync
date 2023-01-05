//go:generate mockgen -destination mock_synctree/mock_synctree.go github.com/anytypeio/any-sync/commonspace/object/tree/synctree SyncClient,SyncTree,ReceiveQueue,HeadNotifiable
package synctree

import (
	"github.com/anytypeio/any-sync/commonspace/confconnector"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/objectsync"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/nodeconf"
)

type SyncClient interface {
	RequestFactory
	BroadcastAsync(message *treechangeproto.TreeSyncMessage) (err error)
	BroadcastAsyncOrSendResponsible(message *treechangeproto.TreeSyncMessage) (err error)
	SendAsync(peerId string, message *treechangeproto.TreeSyncMessage, replyId string) (err error)
}

type syncClient struct {
	objectsync.StreamPool
	RequestFactory
	spaceId       string
	connector     confconnector.ConfConnector
	configuration nodeconf.Configuration

	checker objectsync.StreamChecker
}

func newSyncClient(
	spaceId string,
	pool objectsync.StreamPool,
	factory RequestFactory,
	configuration nodeconf.Configuration,
	checker objectsync.StreamChecker) SyncClient {
	return &syncClient{
		StreamPool:     pool,
		RequestFactory: factory,
		configuration:  configuration,
		checker:        checker,
		spaceId:        spaceId,
	}
}

func (s *syncClient) BroadcastAsync(message *treechangeproto.TreeSyncMessage) (err error) {
	objMsg, err := marshallTreeMessage(message, message.RootChange.Id, "")
	if err != nil {
		return
	}
	s.checker.CheckResponsiblePeers()
	return s.StreamPool.BroadcastAsync(objMsg)
}

func (s *syncClient) SendAsync(peerId string, message *treechangeproto.TreeSyncMessage, replyId string) (err error) {
	err = s.checker.CheckPeerConnection(peerId)
	if err != nil {
		return
	}

	objMsg, err := marshallTreeMessage(message, message.RootChange.Id, replyId)
	if err != nil {
		return
	}
	return s.StreamPool.SendAsync([]string{peerId}, objMsg)
}

func (s *syncClient) BroadcastAsyncOrSendResponsible(message *treechangeproto.TreeSyncMessage) (err error) {
	objMsg, err := marshallTreeMessage(message, message.RootChange.Id, "")
	if err != nil {
		return
	}

	if s.configuration.IsResponsible(s.spaceId) {
		s.checker.CheckResponsiblePeers()
		return s.StreamPool.SendAsync(s.configuration.NodeIds(s.spaceId), objMsg)
	}
	return s.BroadcastAsync(message)
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

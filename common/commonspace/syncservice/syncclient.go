package syncservice

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/ocache"
	"time"
)

type SyncClient interface {
	StreamPool
	RequestFactory
	ocache.ObjectLastUsage
	BroadcastAsyncOrSendResponsible(message *spacesyncproto.ObjectSyncMessage) (err error)
}

type syncClient struct {
	StreamPool
	RequestFactory
	spaceId       string
	notifiable    HeadNotifiable
	configuration nodeconf.Configuration
}

func newSyncClient(spaceId string, pool StreamPool, notifiable HeadNotifiable, factory RequestFactory, configuration nodeconf.Configuration) SyncClient {
	return &syncClient{
		StreamPool:     pool,
		RequestFactory: factory,
		notifiable:     notifiable,
		configuration:  configuration,
		spaceId:        spaceId,
	}
}

func (s *syncClient) LastUsage() time.Time {
	return s.StreamPool.LastUsage()
}

func (s *syncClient) BroadcastAsync(message *spacesyncproto.ObjectSyncMessage) (err error) {
	s.notifyIfNeeded(message)
	return s.StreamPool.BroadcastAsync(message)
}

func (s *syncClient) BroadcastAsyncOrSendResponsible(message *spacesyncproto.ObjectSyncMessage) (err error) {
	if s.configuration.IsResponsible(s.spaceId) {
		return s.SendAsync(s.configuration.NodeIds(s.spaceId), message)
	}
	return s.BroadcastAsync(message)
}

func (s *syncClient) notifyIfNeeded(message *spacesyncproto.ObjectSyncMessage) {
	if message.GetContent().GetHeadUpdate() != nil {
		update := message.GetContent().GetHeadUpdate()
		s.notifiable.UpdateHeads(message.TreeId, update.Heads)
	}
}

//go:generate mockgen -destination mock_syncservice/mock_syncservice.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice SyncClient
package syncservice

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/objectgetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice/synchandler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/rpcerr"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/ocache"
	"time"
)

var log = logger.NewNamed("syncservice").Sugar()

type SyncService interface {
	ocache.ObjectLastUsage
	synchandler.SyncHandler
	SyncClient() SyncClient

	Init(getter objectgetter.ObjectGetter)
	Close() (err error)
}

type HeadNotifiable interface {
	UpdateHeads(id string, heads []string)
}

const respPeersStreamCheckInterval = time.Second * 10

type syncService struct {
	spaceId string

	syncClient    SyncClient
	clientFactory spacesyncproto.ClientFactory
	objectGetter  objectgetter.ObjectGetter

	streamLoopCtx  context.Context
	stopStreamLoop context.CancelFunc
	connector      nodeconf.ConfConnector
	streamLoopDone chan struct{}
}

func NewSyncService(
	spaceId string,
	headNotifiable HeadNotifiable,
	configuration nodeconf.Configuration,
	confConnector nodeconf.ConfConnector) (syncService SyncService) {
	streamPool := newStreamPool(func(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error) {
		return syncService.HandleMessage(ctx, senderId, message)
	})
	factory := newRequestFactory()
	syncClient := newSyncClient(spaceId, streamPool, headNotifiable, factory, configuration)
	syncService = newSyncService(
		spaceId,
		syncClient,
		spacesyncproto.ClientFactoryFunc(spacesyncproto.NewDRPCSpaceClient),
		confConnector)
	return
}

func newSyncService(
	spaceId string,
	syncClient SyncClient,
	clientFactory spacesyncproto.ClientFactory,
	connector nodeconf.ConfConnector) *syncService {
	return &syncService{
		syncClient:     syncClient,
		connector:      connector,
		clientFactory:  clientFactory,
		spaceId:        spaceId,
		streamLoopDone: make(chan struct{}),
	}
}

func (s *syncService) Init(objectGetter objectgetter.ObjectGetter) {
	s.objectGetter = objectGetter
	s.streamLoopCtx, s.stopStreamLoop = context.WithCancel(context.Background())
	go s.responsibleStreamCheckLoop(s.streamLoopCtx)
}

func (s *syncService) Close() (err error) {
	s.stopStreamLoop()
	<-s.streamLoopDone
	return s.syncClient.Close()
}

func (s *syncService) LastUsage() time.Time {
	return s.syncClient.LastUsage()
}

func (s *syncService) HandleMessage(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error) {
	obj, err := s.objectGetter.GetObject(ctx, message.TreeId)
	if err != nil {
		return
	}
	return obj.HandleMessage(ctx, senderId, message)
}

func (s *syncService) responsibleStreamCheckLoop(ctx context.Context) {
	defer close(s.streamLoopDone)
	checkResponsiblePeers := func() {
		respPeers, err := s.connector.DialResponsiblePeers(ctx, s.spaceId)
		if err != nil {
			return
		}
		for _, peer := range respPeers {
			if s.syncClient.HasActiveStream(peer.Id()) {
				continue
			}
			stream, err := s.clientFactory.Client(peer).Stream(ctx)
			if err != nil {
				err = rpcerr.Unwrap(err)
				log.With("spaceId", s.spaceId).Errorf("failed to open stream: %v", err)
				// so here probably the request is failed because there is no such space,
				// but diffService should handle such cases by sending pushSpace
				continue
			}
			// sending empty message for the server to understand from which space is it coming
			err = stream.Send(&spacesyncproto.ObjectSyncMessage{SpaceId: s.spaceId})
			if err != nil {
				err = rpcerr.Unwrap(err)
				log.With("spaceId", s.spaceId).Errorf("failed to send first message to stream: %v", err)
				continue
			}
			s.syncClient.AddAndReadStreamAsync(stream)
		}
	}

	checkResponsiblePeers()
	ticker := time.NewTicker(respPeersStreamCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.streamLoopCtx.Done():
			return
		case <-ticker.C:
			checkResponsiblePeers()
		}
	}
}

func (s *syncService) SyncClient() SyncClient {
	return s.syncClient
}

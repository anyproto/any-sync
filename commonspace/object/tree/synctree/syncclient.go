package synctree

import (
	"context"

	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/sync"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
)

type SyncClient interface {
	RequestFactory
	Broadcast(ctx context.Context, headUpdate HeadUpdate) error
	SendTreeRequest(ctx context.Context, req syncdeps.Request, collector syncdeps.ResponseCollector) (err error)
	QueueRequest(ctx context.Context, peerId string, tree objecttree.ObjectTree) (err error)
}

type syncClient struct {
	RequestFactory
	syncService sync.SyncService
	spaceId     string
}

func NewSyncClient(spaceId string, syncService sync.SyncService) SyncClient {
	return &syncClient{
		RequestFactory: &requestFactory{},
		spaceId:        spaceId,
		syncService:    syncService,
	}
}

func (s *syncClient) Broadcast(ctx context.Context, headUpdate HeadUpdate) error {
	return s.syncService.BroadcastMessage(ctx, headUpdate)
}

func (s *syncClient) SendTreeRequest(ctx context.Context, req syncdeps.Request, collector syncdeps.ResponseCollector) (err error) {
	return s.syncService.SendRequest(ctx, req, collector)
}

func (s *syncClient) QueueRequest(ctx context.Context, peerId string, tree objecttree.ObjectTree) (err error) {
	req := s.CreateFullSyncRequest(peerId, tree)
	return s.syncService.QueueRequest(ctx, req)
}

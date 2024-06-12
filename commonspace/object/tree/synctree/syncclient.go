package synctree

import (
	"context"

	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/sync"
)

type SyncClient interface {
	RequestFactory
	Broadcast(ctx context.Context, headUpdate HeadUpdate) error
	SendNewTreeRequest(ctx context.Context, peerId, objectId string, collector *responseCollector) (err error)
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

func (s *syncClient) SendNewTreeRequest(ctx context.Context, peerId, objectId string, collector *responseCollector) (err error) {
	req := s.CreateNewTreeRequest(peerId, objectId)
	return s.syncService.SendRequest(ctx, req, collector)
}

func (s *syncClient) QueueRequest(ctx context.Context, peerId string, tree objecttree.ObjectTree) (err error) {
	req := s.CreateFullSyncRequest(peerId, tree)
	return s.syncService.QueueRequest(ctx, req)
}

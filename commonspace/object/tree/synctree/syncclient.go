package synctree

import (
	"context"

	"github.com/anyproto/any-sync/commonspace/sync"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
)

type SyncClient interface {
	RequestFactory
	Broadcast(ctx context.Context, headUpdate *objectmessages.HeadUpdate) error
	SendTreeRequest(ctx context.Context, req syncdeps.Request, collector syncdeps.ResponseCollector) (err error)
	QueueRequest(ctx context.Context, req syncdeps.Request) (err error)
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

func (s *syncClient) Broadcast(ctx context.Context, headUpdate *objectmessages.HeadUpdate) error {
	return s.syncService.BroadcastMessage(ctx, headUpdate)
}

func (s *syncClient) SendTreeRequest(ctx context.Context, req syncdeps.Request, collector syncdeps.ResponseCollector) (err error) {
	return s.syncService.SendRequest(ctx, req, collector)
}

func (s *syncClient) QueueRequest(ctx context.Context, req syncdeps.Request) (err error) {
	return s.syncService.QueueRequest(ctx, req)
}

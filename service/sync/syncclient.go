package sync

import "github.com/anytypeio/go-anytype-infrastructure-experiments/service/sync/syncpb"

type SyncClient interface {
	NotifyHeadsChanged(update *syncpb.SyncHeadUpdate) error
	RequestFullSync(id string, request *syncpb.SyncFullRequest) error
}

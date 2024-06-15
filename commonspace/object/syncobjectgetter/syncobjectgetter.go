package syncobjectgetter

import (
	"context"

	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
)

type SyncObject interface {
	Id() string
	syncdeps.ObjectSyncHandler
}

type SyncObjectGetter interface {
	GetObject(ctx context.Context, objectId string) (SyncObject, error)
}

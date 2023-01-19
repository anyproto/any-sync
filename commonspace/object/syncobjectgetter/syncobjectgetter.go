package syncobjectgetter

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/objectsync/synchandler"
)

type SyncObject interface {
	Id() string
	synchandler.SyncHandler
}

type SyncObjectGetter interface {
	GetObject(ctx context.Context, objectId string) (SyncObject, error)
}

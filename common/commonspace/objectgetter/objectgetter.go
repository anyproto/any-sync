package objectgetter

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice/synchandler"
)

type Object interface {
	synchandler.SyncHandler
}

type ObjectGetter interface {
	GetObject(ctx context.Context, objectId string) (Object, error)
}

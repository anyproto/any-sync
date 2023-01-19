package streammanager

import (
	"context"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/commonspace/objectsync"
)

const CName = "common.commonspace.streammanager"

type StreamManagerProvider interface {
	app.Component
	NewStreamManager(ctx context.Context, spaceId string) (sm objectsync.StreamManager, err error)
}

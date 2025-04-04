package kvinterfaces

import (
	"context"

	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage"
)

const CName = "common.object.keyvalue"

type KeyValueService interface {
	app.ComponentRunnable
	DefaultStore() keyvaluestorage.Storage
	HandleMessage(ctx context.Context, msg drpc.Message) (err error)
}
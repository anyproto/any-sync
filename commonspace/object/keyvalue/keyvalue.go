package keyvalue

import (
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage/innerstorage"
)

type KeyValueService interface {
	app.ComponentRunnable
	DefaultStore() innerstorage.KeyValueStorage
}

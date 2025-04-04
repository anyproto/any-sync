package keyvalue

import (
	"context"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage/innerstorage"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
)

const CName = "common.object.keyvalue"

type KeyValueService interface {
	app.ComponentRunnable
	DefaultStore() innerstorage.KeyValueStorage
}

type keyValueService struct {
	spaceStorage spacestorage.SpaceStorage
	defaultStore keyvaluestorage.Storage
}

func (k *keyValueService) Init(a *app.App) (err error) {
	//TODO implement me
	panic("implement me")
}

func (k *keyValueService) Name() (name string) {
	return CName
}

func (k *keyValueService) Run(ctx context.Context) (err error) {
	//TODO implement me
	panic("implement me")
}

func (k *keyValueService) Close(ctx context.Context) (err error) {
	//TODO implement me
	panic("implement me")
}

package keyvaluestorage

import (
	"context"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage/innerstorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type Indexer interface {
	Index(keyValue innerstorage.KeyValue) error
}

type Storage interface {
	Set(ctx context.Context, key string, value []byte) error
	SetRaw(ctx context.Context, keyValue *spacesyncproto.StoreKeyValue) error
	GetAll(ctx context.Context, key string) (values []innerstorage.KeyValue, err error)
	InnerStorage() innerstorage.KeyValueStorage
}

type storage struct {
	inner   innerstorage.KeyValueStorage
	keys    *accountdata.AccountKeys
	aclList list.AclList
}

func (s *storage) Set(ctx context.Context, key string, value []byte) error {
	//TODO implement me
	panic("implement me")
}

func (s *storage) SetRaw(ctx context.Context, keyValue *spacesyncproto.StoreKeyValue) error {
	//TODO implement me
	panic("implement me")
}

func (s *storage) GetAll(ctx context.Context, key string) (values []innerstorage.KeyValue, err error) {
	err = s.inner.IteratePrefix(ctx, key, func(kv innerstorage.KeyValue) error {
		values = append(values, kv)
		return nil
	})
	return
}

func (s *storage) InnerStorage() innerstorage.KeyValueStorage {
	return s.inner
}

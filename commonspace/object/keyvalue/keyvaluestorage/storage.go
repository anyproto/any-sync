package keyvaluestorage

import (
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage/innerstorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type Storage interface {
	Set(key string, value []byte) error
	SetRaw(keyValue *spacesyncproto.StoreKeyValue) error
	GetAll(key string) (values []innerstorage.KeyValue, err error)
	InnerStorage() innerstorage.KeyValueStorage
}
//
//type storage struct {
//
//}
//
//func (s *storage) Set(key string, value []byte) error {
//	//TODO implement me
//	panic("implement me")
//}
//
//func (s *storage) SetRaw(keyValue *spacesyncproto.StoreKeyValue) error {
//	//TODO implement me
//	panic("implement me")
//}
//
//func (s *storage) GetAll(key string) (values []innerstorage.KeyValue, err error) {
//	//TODO implement me
//	panic("implement me")
//}
//
//func (s *storage) InnerStorage() innerstorage.KeyValueStorage {
//	//TODO implement me
//	panic("implement me")
//}

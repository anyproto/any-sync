package keyvaluestorage

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage/innerstorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

var log = logger.NewNamed("common.keyvalue.keyvaluestorage")

type Indexer interface {
	Index(keyValue ...innerstorage.KeyValue) error
}

type Storage interface {
	Set(ctx context.Context, key string, value []byte) error
	SetRaw(ctx context.Context, keyValue ...*spacesyncproto.StoreKeyValue) error
	GetAll(ctx context.Context, key string) (values []innerstorage.KeyValue, err error)
	InnerStorage() innerstorage.KeyValueStorage
}

type storage struct {
	inner   innerstorage.KeyValueStorage
	keys    *accountdata.AccountKeys
	aclList list.AclList
	indexer Indexer
	mx      sync.Mutex
}

func (s *storage) Set(ctx context.Context, key string, value []byte) error {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.aclList.RLock()
	headId := s.aclList.Head().Id
	if !s.aclList.AclState().Permissions(s.aclList.AclState().Identity()).CanWrite() {
		s.aclList.RUnlock()
		return list.ErrInsufficientPermissions
	}
	s.aclList.RUnlock()
	peerIdKey := s.keys.PeerKey
	identityKey := s.keys.SignKey
	protoPeerKey, err := peerIdKey.GetPublic().Marshall()
	if err != nil {
		return err
	}
	protoIdentityKey, err := identityKey.GetPublic().Marshall()
	if err != nil {
		return err
	}
	timestampMilli := time.Now().UnixMilli()
	inner := spacesyncproto.StoreKeyInner{
		Peer:           protoPeerKey,
		Identity:       protoIdentityKey,
		Value:          value,
		TimestampMilli: timestampMilli,
		AclHeadId:      headId,
		Key:            key,
	}
	innerBytes, err := inner.Marshal()
	if err != nil {
		return err
	}
	peerSig, err := peerIdKey.Sign(innerBytes)
	if err != nil {
		return err
	}
	identitySig, err := identityKey.Sign(innerBytes)
	if err != nil {
		return err
	}
	keyValue := innerstorage.KeyValue{
		Key: key,
		Value: innerstorage.Value{
			Value:             value,
			PeerSignature:     peerSig,
			IdentitySignature: identitySig,
		},
		TimestampMilli: int(timestampMilli),
		Identity:       identityKey.GetPublic().Account(),
		PeerId:         peerIdKey.GetPublic().PeerId(),
	}
	err = s.inner.Set(ctx, keyValue)
	if err != nil {
		return err
	}
	indexErr := s.indexer.Index(keyValue)
	if indexErr != nil {
		log.Warn("failed to index for key", zap.String("key", key), zap.Error(indexErr))
	}
	return nil
}

func (s *storage) SetRaw(ctx context.Context, keyValue ...*spacesyncproto.StoreKeyValue) error {
	s.mx.Lock()
	defer s.mx.Unlock()
	keyValues := make([]innerstorage.KeyValue, len(keyValue))
	for _, kv := range keyValue {
		innerKv, err := innerstorage.KeyValueFromProto(kv, true)
		if err != nil {
			return err
		}
		keyValues = append(keyValues, innerKv)
	}
	err := s.inner.Set(ctx, keyValues...)
	if err != nil {
		return err
	}
	indexErr := s.indexer.Index(keyValues...)
	if indexErr != nil {
		log.Warn("failed to index for key", zap.String("key", key), zap.Error(indexErr))
	}
	return nil
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

package keyvaluestorage

import (
	"context"
	"sync"
	"time"

	anystore "github.com/anyproto/any-store"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage/innerstorage"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage/syncstorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

var log = logger.NewNamed("common.keyvalue.keyvaluestorage")

const IndexerCName = "common.keyvalue.indexer"

type Indexer interface {
	app.Component
	Index(keyValue ...innerstorage.KeyValue) error
}

type NoOpIndexer struct{}

func (n NoOpIndexer) Init(a *app.App) (err error) {
	return nil
}

func (n NoOpIndexer) Name() (name string) {
	return IndexerCName
}

func (n NoOpIndexer) Index(keyValue ...innerstorage.KeyValue) error {
	return nil
}

type Storage interface {
	Id() string
	Set(ctx context.Context, key string, value []byte) error
	SetRaw(ctx context.Context, keyValue ...*spacesyncproto.StoreKeyValue) error
	GetAll(ctx context.Context, key string) (values []innerstorage.KeyValue, err error)
	InnerStorage() innerstorage.KeyValueStorage
}

type storage struct {
	inner      innerstorage.KeyValueStorage
	keys       *accountdata.AccountKeys
	aclList    list.AclList
	syncClient syncstorage.SyncClient
	indexer    Indexer
	storageId  string
	mx         sync.Mutex
}

func New(
	ctx context.Context,
	storageId string,
	store anystore.DB,
	headStorage headstorage.HeadStorage,
	keys *accountdata.AccountKeys,
	syncClient syncstorage.SyncClient,
	aclList list.AclList,
	indexer Indexer,
) (Storage, error) {
	inner, err := innerstorage.New(ctx, storageId, headStorage, store)
	if err != nil {
		return nil, err
	}
	return &storage{
		inner:      inner,
		keys:       keys,
		storageId:  storageId,
		aclList:    aclList,
		indexer:    indexer,
		syncClient: syncClient,
	}, nil
}

func (s *storage) Id() string {
	return s.storageId
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
	keyPeerId := key + "-" + peerIdKey.GetPublic().PeerId()
	keyValue := innerstorage.KeyValue{
		KeyPeerId:      keyPeerId,
		Key:            key,
		TimestampMilli: int(timestampMilli),
		Identity:       identityKey.GetPublic().Account(),
		PeerId:         peerIdKey.GetPublic().PeerId(),
		Value: innerstorage.Value{
			Value:             value,
			PeerSignature:     peerSig,
			IdentitySignature: identitySig,
		},
	}
	err = s.inner.Set(ctx, keyValue)
	if err != nil {
		return err
	}
	indexErr := s.indexer.Index(keyValue)
	if indexErr != nil {
		log.Warn("failed to index for key", zap.String("key", key), zap.Error(indexErr))
	}
	sendErr := s.syncClient.Broadcast(ctx, s.storageId, keyValue)
	if sendErr != nil {
		log.Warn("failed to send key value", zap.String("key", key), zap.Error(sendErr))
	}
	return nil
}

func (s *storage) SetRaw(ctx context.Context, keyValue ...*spacesyncproto.StoreKeyValue) error {
	if len(keyValue) == 0 {
		return nil
	}
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
		log.Warn("failed to index for keys", zap.Error(indexErr))
	}
	sendErr := s.syncClient.Broadcast(ctx, s.storageId, keyValues...)
	if sendErr != nil {
		log.Warn("failed to send key values", zap.Error(sendErr))
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

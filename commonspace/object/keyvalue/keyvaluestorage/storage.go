package keyvaluestorage

import (
	"context"
	"fmt"
	"sync"
	"time"

	anystore "github.com/anyproto/any-store"
	"github.com/anyproto/protobuf/proto"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage/innerstorage"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage/syncstorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/anyproto/any-sync/util/slice"
)

var log = logger.NewNamed("common.keyvalue.keyvaluestorage")

const IndexerCName = "common.keyvalue.indexer"

type Indexer interface {
	app.Component
	Index(decryptor Decryptor, keyValue ...innerstorage.KeyValue) error
}

type Decryptor = func(kv innerstorage.KeyValue) (value []byte, err error)

type NoOpIndexer struct{}

func (n NoOpIndexer) Init(a *app.App) (err error) {
	return nil
}

func (n NoOpIndexer) Name() (name string) {
	return IndexerCName
}

func (n NoOpIndexer) Index(decryptor Decryptor, keyValue ...innerstorage.KeyValue) error {
	return nil
}

type Storage interface {
	Id() string
	Set(ctx context.Context, key string, value []byte) error
	SetRaw(ctx context.Context, keyValue ...*spacesyncproto.StoreKeyValue) error
	GetAll(ctx context.Context, key string) (values []innerstorage.KeyValue, err error)
	Iterate(ctx context.Context, f func(key string, values []innerstorage.KeyValue) (bool, error)) error
	InnerStorage() innerstorage.KeyValueStorage
}

type storage struct {
	inner          innerstorage.KeyValueStorage
	keys           *accountdata.AccountKeys
	aclList        list.AclList
	syncClient     syncstorage.SyncClient
	indexer        Indexer
	storageId      string
	readKeys       map[string]crypto.SymKey
	currentReadKey crypto.SymKey
	mx             sync.Mutex
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
		readKeys:   make(map[string]crypto.SymKey),
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
	state := s.aclList.AclState()
	if !s.aclList.AclState().Permissions(state.Identity()).CanWrite() {
		s.aclList.RUnlock()
		return list.ErrInsufficientPermissions
	}
	readKeyId := state.CurrentReadKeyId()
	err := s.readKeysFromAclState(state)
	if err != nil {
		s.aclList.RUnlock()
		return err
	}
	s.aclList.RUnlock()
	value, err = s.currentReadKey.Encrypt(value)
	if err != nil {
		return err
	}
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
		AclId:          headId,
		ReadKeyId:      readKeyId,
		Value: innerstorage.Value{
			Value:             innerBytes,
			PeerSignature:     peerSig,
			IdentitySignature: identitySig,
		},
	}
	err = s.inner.Set(ctx, keyValue)
	if err != nil {
		return err
	}
	indexErr := s.indexer.Index(s.decrypt, keyValue)
	if indexErr != nil {
		log.Warn("failed to index for key", zap.String("key", key), zap.Error(indexErr))
	}
	sendErr := s.syncClient.Broadcast(ctx, s.storageId, keyValue)
	if sendErr != nil {
		log.Warn("failed to send key value", zap.String("key", key), zap.Error(sendErr))
	}
	return nil
}

func (s *storage) SetRaw(ctx context.Context, keyValue ...*spacesyncproto.StoreKeyValue) (err error) {
	if len(keyValue) == 0 {
		return nil
	}
	s.mx.Lock()
	defer s.mx.Unlock()
	keyValues := make([]innerstorage.KeyValue, 0, len(keyValue))
	for _, kv := range keyValue {
		innerKv, err := innerstorage.KeyValueFromProto(kv, true)
		if err != nil {
			return err
		}
		keyValues = append(keyValues, innerKv)
	}
	s.aclList.RLock()
	state := s.aclList.AclState()
	err = s.readKeysFromAclState(state)
	if err != nil {
		s.aclList.RUnlock()
		return err
	}
	for i := range keyValues {
		keyValues[i].ReadKeyId, err = state.ReadKeyForAclId(keyValues[i].AclId)
		if err != nil {
			keyValues[i].KeyPeerId = ""
			continue
		}
	}
	s.aclList.RUnlock()
	keyValues = slice.DiscardFromSlice(keyValues, func(value innerstorage.KeyValue) bool {
		return value.KeyPeerId == ""
	})
	err = s.inner.Set(ctx, keyValues...)
	if err != nil {
		return err
	}
	sendErr := s.syncClient.Broadcast(ctx, s.storageId, keyValues...)
	if sendErr != nil {
		log.Warn("failed to send key values", zap.Error(sendErr))
	}
	indexErr := s.indexer.Index(s.decrypt, keyValues...)
	if indexErr != nil {
		log.Warn("failed to index for keys", zap.Error(indexErr))
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

func (s *storage) readKeysFromAclState(state *list.AclState) (err error) {
	if len(s.readKeys) == len(state.Keys()) {
		return nil
	}
	if state.AccountKey() == nil || !state.HadReadPermissions(state.AccountKey().GetPublic()) {
		return nil
	}
	for key, value := range state.Keys() {
		if _, exists := s.readKeys[key]; exists {
			continue
		}
		if value.ReadKey == nil {
			continue
		}
		treeKey, err := deriveKey(value.ReadKey, s.storageId)
		if err != nil {
			return err
		}
		s.readKeys[key] = treeKey
	}
	curKey, err := state.CurrentReadKey()
	if err != nil {
		return err
	}
	if curKey == nil {
		return nil
	}
	s.currentReadKey, err = deriveKey(curKey, s.storageId)
	return err
}

func (s *storage) Iterate(ctx context.Context, f func(key string, values []innerstorage.KeyValue) (bool, error)) (err error) {
	var (
		curKey = ""
		values []innerstorage.KeyValue
	)
	err = s.inner.IterateValues(ctx, func(kv innerstorage.KeyValue) (bool, error) {
		if kv.Key != curKey {
			if curKey != "" {
				iter, err := f(curKey, values)
				if err != nil {
					return false, err
				}
				if !iter {
					return false, nil
				}
			}
			curKey = kv.Key
			values = values[:0]
		}
		values = append(values, kv)
		return true, nil
	})
	if err != nil {
		return err
	}
	if len(values) > 0 {
		_, err = f(curKey, values)
	}
	return err
}

func (s *storage) decrypt(kv innerstorage.KeyValue) (value []byte, err error) {
	if kv.ReadKeyId == "" {
		return nil, fmt.Errorf("no read key id")
	}
	key := s.readKeys[kv.ReadKeyId]
	if key == nil {
		return nil, fmt.Errorf("no read key for %s", kv.ReadKeyId)
	}
	msg := &spacesyncproto.StoreKeyInner{}
	err = proto.Unmarshal(kv.Value.Value, msg)
	if err != nil {
		return nil, err
	}
	value, err = key.Decrypt(msg.Value)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func deriveKey(key crypto.SymKey, id string) (crypto.SymKey, error) {
	raw, err := key.Raw()
	if err != nil {
		return nil, err
	}
	return crypto.DeriveSymmetricKey(raw, fmt.Sprintf(crypto.AnysyncKeyValuePath, id))
}

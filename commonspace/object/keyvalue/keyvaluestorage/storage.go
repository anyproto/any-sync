//go:generate mockgen -destination mock_keyvaluestorage/mock_keyvaluestorage.go github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage Storage
package keyvaluestorage

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
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
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/anyproto/any-sync/util/slice"
)

var log = logger.NewNamed("common.keyvalue.keyvaluestorage")

var (
	// ErrCoveredByWatermark: the write's key sits under a deletion watermark
	// newer than the write's timestamp, so every replica would discard it.
	ErrCoveredByWatermark = errors.New("key is covered by a newer deletion watermark")
	ErrEmptyDeletePrefix  = errors.New("empty delete prefix")
)

const IndexerCName = "common.keyvalue.indexer"

type Indexer interface {
	app.Component
	Index(decryptor Decryptor, keyValue ...innerstorage.KeyValue) error
}

// DeletionAwareIndexer is implemented by indexers that mirror physical
// deletions: RemoveIndex receives the ids of rows a watermark dropped, so the
// downstream index does not keep resolving keys the storage no longer holds.
type DeletionAwareIndexer interface {
	Indexer
	RemoveIndex(keyPeerIds ...string) error
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
	Prepare() error
	Set(ctx context.Context, key string, value []byte) error
	SetRaw(ctx context.Context, keyValue ...*spacesyncproto.StoreKeyValue) error
	// DeletePrefix publishes a deletion watermark: every row whose key
	// starts with prefix and predates the watermark is physically dropped
	// on every replica, and stays rejected should it arrive again (e.g.
	// from a device restoring an old snapshot). The watermark row is the
	// only retained state. Owner-only.
	DeletePrefix(ctx context.Context, prefix string) error
	GetAll(ctx context.Context, key string, get func(decryptor Decryptor, values []innerstorage.KeyValue) error) error
	Iterate(ctx context.Context, f func(decryptor Decryptor, key string, values []innerstorage.KeyValue) (bool, error)) error
	InnerStorage() innerstorage.KeyValueStorage
}

type storage struct {
	inner          innerstorage.KeyValueStorage
	keys           *accountdata.AccountKeys
	aclList        list.AclList
	syncClient     syncstorage.SyncClient
	indexer        Indexer
	storageId      string
	byteRepr       []byte
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
	s := &storage{
		inner:      inner,
		keys:       keys,
		storageId:  storageId,
		aclList:    aclList,
		indexer:    indexer,
		syncClient: syncClient,
		byteRepr:   make([]byte, 8),
		readKeys:   make(map[string]crypto.SymKey),
	}
	return s, nil
}

func (s *storage) Prepare() error {
	s.aclList.RLock()
	defer s.aclList.RUnlock()
	return s.readKeysFromAclState(s.aclList.AclState())
}

func (s *storage) Id() string {
	return s.storageId
}

func (s *storage) Set(ctx context.Context, key string, value []byte) error {
	s.mx.Lock()
	defer s.mx.Unlock()
	headId, readKeyId, err := s.prepareWrite(func(p list.AclPermissions) bool { return p.CanWrite() })
	if err != nil {
		return err
	}
	value, err = s.currentReadKey.Encrypt(value)
	if err != nil {
		return err
	}
	return s.signAndStore(ctx, key, value, "", headId, readKeyId)
}

func (s *storage) DeletePrefix(ctx context.Context, prefix string) error {
	if prefix == "" {
		return ErrEmptyDeletePrefix
	}
	s.mx.Lock()
	defer s.mx.Unlock()
	// Owner-only: a watermark removes rows written by every peer, not just
	// the caller's own.
	headId, readKeyId, err := s.prepareWrite(func(p list.AclPermissions) bool { return p.IsOwner() })
	if err != nil {
		return err
	}
	return s.signAndStore(ctx, prefix, nil, prefix, headId, readKeyId)
}

// prepareWrite runs the shared ACL section of a local write: permission
// check, read-key refresh, and the current head/read-key ids the row is
// stamped with.
func (s *storage) prepareWrite(allowed func(list.AclPermissions) bool) (headId, readKeyId string, err error) {
	s.aclList.RLock()
	defer s.aclList.RUnlock()
	headId = s.aclList.Head().Id
	state := s.aclList.AclState()
	if !allowed(state.Permissions(state.Identity())) {
		return "", "", list.ErrInsufficientPermissions
	}
	readKeyId = state.CurrentReadKeyId()
	if err = s.readKeysFromAclState(state); err != nil {
		return "", "", err
	}
	return headId, readKeyId, nil
}

// signAndStore builds, signs, applies and broadcasts one own row: a regular
// value (encrypted by the caller) or, with deletePrefix set, a deletion
// watermark (empty value, key mirrors the prefix).
func (s *storage) signAndStore(ctx context.Context, key string, value []byte, deletePrefix string, headId, readKeyId string) error {
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
	timestampMicro := time.Now().UnixMicro()
	// Fail loudly instead of writing a row every replica (this one included)
	// would silently discard: without this, Set would return success and the
	// value would never be readable anywhere.
	if wmTs := s.inner.WatermarkTs(key); wmTs > timestampMicro {
		return ErrCoveredByWatermark
	}
	inner := spacesyncproto.StoreKeyInner{
		Peer:           protoPeerKey,
		Identity:       protoIdentityKey,
		Value:          value,
		TimestampMicro: timestampMicro,
		AclHeadId:      headId,
		Key:            key,
		DeletePrefix:   deletePrefix,
	}
	innerBytes, err := inner.MarshalVT()
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
		TimestampMicro: timestampMicro,
		Identity:       identityKey.GetPublic().Account(),
		PeerId:         peerIdKey.GetPublic().PeerId(),
		AclId:          headId,
		ReadKeyId:      readKeyId,
		DeletePrefix:   deletePrefix,
		Value: innerstorage.Value{
			Value:             innerBytes,
			PeerSignature:     peerSig,
			IdentitySignature: identitySig,
		},
	}
	res, err := s.inner.Set(ctx, keyValue)
	if err != nil {
		return err
	}
	s.removeIndexes(res.DroppedIds)
	if len(res.Applied) == 0 {
		// Lost LWW to an already-stored newer own row; nothing changed.
		return nil
	}
	if deletePrefix == "" {
		indexErr := s.indexer.Index(s.decrypt, keyValue)
		if indexErr != nil {
			log.Warn("failed to index for key", zap.String("key", key), zap.Error(indexErr))
		}
	}
	sendErr := s.syncClient.Broadcast(ctx, s.storageId, keyValue)
	if sendErr != nil {
		log.Warn("failed to send key value", zap.String("key", key), zap.Error(sendErr))
	}
	return nil
}

// removeIndexes mirrors watermark drops into the indexer when it opts in.
func (s *storage) removeIndexes(droppedIds []string) {
	if len(droppedIds) == 0 {
		return
	}
	indexer, ok := s.indexer.(DeletionAwareIndexer)
	if !ok {
		return
	}
	if err := indexer.RemoveIndex(droppedIds...); err != nil {
		log.Warn("failed to remove index for dropped keys", zap.Error(err))
	}
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
			// A value that fails to decode or verify poisons only itself, like the
			// ACL-skip below: aborting the whole call would let one bad element
			// block its entire batch — and wedge every pull at the same point,
			// since values arrive in a deterministic order.
			log.Warn("skipping invalid key value", zap.String("key", kv.KeyPeerId), zap.Error(err))
			continue
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
		// A watermark deletes other peers' rows, so accepting one demands
		// the owner's identity — the same bar DeletePrefix applies locally.
		if keyValues[i].DeletePrefix != "" && !state.Permissions(keyValues[i].IdentityPubKey).IsOwner() {
			keyValues[i].KeyPeerId = ""
			continue
		}
		el, err := s.inner.Diff().Element(keyValues[i].KeyPeerId)
		if err == nil {
			binary.BigEndian.PutUint64(s.byteRepr, uint64(keyValues[i].TimestampMicro))
			if el.Head >= string(s.byteRepr) {
				keyValues[i].KeyPeerId = ""
				continue
			}
		}
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
	if len(keyValues) == 0 {
		return nil
	}
	res, err := s.inner.Set(ctx, keyValues...)
	if err != nil {
		return err
	}
	s.removeIndexes(res.DroppedIds)
	// Broadcast and index only what actually applied: values the store
	// rejected (LWW losers, watermark-covered rows) must not propagate
	// further — indexing them would resurrect deleted data downstream.
	if len(res.Applied) == 0 {
		return nil
	}
	sendErr := s.syncClient.Broadcast(ctx, s.storageId, res.Applied...)
	if sendErr != nil {
		log.Warn("failed to send key values", zap.Error(sendErr))
	}
	// Watermarks carry no payload to index; their effect (dropped rows) is
	// already applied.
	indexable := slice.DiscardFromSlice(res.Applied, func(value innerstorage.KeyValue) bool {
		return value.DeletePrefix != ""
	})
	if len(indexable) > 0 {
		indexErr := s.indexer.Index(s.decrypt, indexable...)
		if indexErr != nil {
			log.Warn("failed to index for keys", zap.Error(indexErr))
		}
	}
	return nil
}

func (s *storage) GetAll(ctx context.Context, key string, get func(decryptor Decryptor, values []innerstorage.KeyValue) error) (err error) {
	var values []innerstorage.KeyValue
	err = s.inner.IteratePrefix(ctx, key, func(kv innerstorage.KeyValue) error {
		if kv.DeletePrefix != "" {
			return nil
		}
		values = append(values, kv)
		return nil
	})
	if err != nil {
		return err
	}
	s.mx.Lock()
	defer s.mx.Unlock()
	return get(s.decrypt, values)
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
	curKeyId := state.CurrentReadKeyId()
	if derived, ok := s.readKeys[curKeyId]; ok {
		s.currentReadKey = derived
		return nil
	}
	// Fallback: derive if not in map (e.g., ReadKey was nil and skipped in the loop above)
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

func (s *storage) Iterate(ctx context.Context, f func(decryptor Decryptor, key string, values []innerstorage.KeyValue) (bool, error)) (err error) {
	s.mx.Lock()
	defer s.mx.Unlock()
	var (
		curKey = ""
		// TODO: reuse buffer
		values []innerstorage.KeyValue
	)
	err = s.inner.IterateValues(ctx, func(kv innerstorage.KeyValue) (bool, error) {
		if kv.DeletePrefix != "" {
			return true, nil
		}
		if kv.Key != curKey {
			if curKey != "" {
				iter, err := f(s.decrypt, curKey, values)
				if err != nil {
					return false, err
				}
				if !iter {
					values = nil
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
		_, err = f(s.decrypt, curKey, values)
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
	err = msg.UnmarshalVT(kv.Value.Value)
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

package storage

import (
	"context"
	"errors"
	provider "github.com/anytypeio/go-anytype-infrastructure-experiments/client/badgerprovider"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/dgraph-io/badger/v3"
)

var ErrIncorrectKey = errors.New("key format is incorrect")

type listStorage struct {
	db   *badger.DB
	keys aclKeys
	id   string
	root *aclrecordproto.RawACLRecordWithId
}

func newListStorage(spaceId string, db *badger.DB, txn *badger.Txn) (ls storage.ListStorage, err error) {
	keys := newACLKeys(spaceId)
	rootId, err := provider.GetKeySuffix(txn, keys.RootIdKey())
	if err != nil {
		if err == badger.ErrKeyNotFound {
			err = storage.ErrUnknownACLId
		}
		return
	}
	stringId := string(rootId)
	rootItem, err := txn.Get(keys.RawRecordKey(stringId))
	if err != nil {
		if err == badger.ErrKeyNotFound {
			err = storage.ErrUnknownACLId
		}
		return
	}
	var value []byte
	value, err = rootItem.ValueCopy(value)
	if err != nil {
		return
	}

	rootWithId := &aclrecordproto.RawACLRecordWithId{
		Payload: value,
		Id:      stringId,
	}

	ls = &listStorage{
		db:   db,
		keys: aclKeys{},
		id:   stringId,
		root: rootWithId,
	}
	return
}

func createListStorage(spaceId string, db *badger.DB, txn *badger.Txn, root *aclrecordproto.RawACLRecordWithId) (ls storage.ListStorage, err error) {
	keys := newACLKeys(spaceId)
	_, err = provider.GetKeySuffix(txn, keys.RootIdKey())
	if err != nil && err != badger.ErrKeyNotFound {
		return
	}
	if err == nil {
		err = storage.ErrACLExists
		return
	}
	aclRootKey := append(keys.RootIdKey(), '/')
	aclRootKey = append(aclRootKey, []byte(root.Id)...)

	err = txn.Set(aclRootKey, nil)
	if err != nil {
		return
	}

	err = txn.Set(keys.HeadIdKey(), []byte(root.Id))
	if err != nil {
		return
	}

	err = txn.Set(keys.RawRecordKey(root.Id), root.Payload)
	if err != nil {
		return
	}

	ls = &listStorage{
		db:   db,
		keys: aclKeys{},
		id:   root.Id,
		root: root,
	}
	return
}

func (l *listStorage) ID() (string, error) {
	return l.id, nil
}

func (l *listStorage) Root() (*aclrecordproto.RawACLRecordWithId, error) {
	return l.root, nil
}

func (l *listStorage) Head() (head string, err error) {
	bytes, err := provider.Get(l.db, l.keys.HeadIdKey())
	if err != nil {
		return
	}
	head = string(bytes)
	return
}

func (l *listStorage) GetRawRecord(ctx context.Context, id string) (raw *aclrecordproto.RawACLRecordWithId, err error) {
	res, err := l.db.Get(l.keys.RawRecordKey(id))
	if err != nil {
		return
	}
	if res == nil {
		err = storage.ErrUnknownRecord
		return
	}

	raw = &aclrecordproto.RawACLRecordWithId{
		Payload: res,
		Id:      id,
	}
	return
}

func (l *listStorage) AddRawRecord(ctx context.Context, rec *aclrecordproto.RawACLRecordWithId) error {
	return l.db.Put([]byte(rec.Id), rec.Payload)
}

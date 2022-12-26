package storage

import (
	"context"
	"github.com/akrylysov/pogreb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/liststorage"
)

type listStorage struct {
	db   *pogreb.DB
	keys aclKeys
	id   string
	root *aclrecordproto.RawAclRecordWithId
}

func newListStorage(db *pogreb.DB) (ls liststorage.ListStorage, err error) {
	keys := aclKeys{}
	rootId, err := db.Get(keys.RootIdKey())
	if err != nil {
		return
	}
	if rootId == nil {
		err = liststorage.ErrUnknownAclId
		return
	}

	root, err := db.Get(keys.RawRecordKey(string(rootId)))
	if err != nil {
		return
	}
	if root == nil {
		err = liststorage.ErrUnknownAclId
		return
	}

	rootWithId := &aclrecordproto.RawAclRecordWithId{
		Payload: root,
		Id:      string(rootId),
	}

	ls = &listStorage{
		db:   db,
		keys: aclKeys{},
		id:   string(rootId),
		root: rootWithId,
	}
	return
}

func createListStorage(db *pogreb.DB, root *aclrecordproto.RawAclRecordWithId) (ls liststorage.ListStorage, err error) {
	keys := aclKeys{}
	has, err := db.Has(keys.RootIdKey())
	if err != nil {
		return
	}
	if has {
		return newListStorage(db)
	}

	err = db.Put(keys.HeadIdKey(), []byte(root.Id))
	if err != nil {
		return
	}

	err = db.Put(keys.RawRecordKey(root.Id), root.Payload)
	if err != nil {
		return
	}

	err = db.Put(keys.RootIdKey(), []byte(root.Id))
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

func (l *listStorage) Id() string {
	return l.id
}

func (l *listStorage) Root() (*aclrecordproto.RawAclRecordWithId, error) {
	return l.root, nil
}

func (l *listStorage) Head() (head string, err error) {
	bytes, err := l.db.Get(l.keys.HeadIdKey())
	if err != nil {
		return
	}
	if bytes == nil {
		err = liststorage.ErrUnknownAclId
		return
	}
	head = string(bytes)
	return
}

func (l *listStorage) GetRawRecord(ctx context.Context, id string) (raw *aclrecordproto.RawAclRecordWithId, err error) {
	res, err := l.db.Get(l.keys.RawRecordKey(id))
	if err != nil {
		return
	}
	if res == nil {
		err = liststorage.ErrUnknownRecord
		return
	}

	raw = &aclrecordproto.RawAclRecordWithId{
		Payload: res,
		Id:      id,
	}
	return
}

func (l *listStorage) SetHead(headId string) (err error) {
	return l.db.Put(l.keys.HeadIdKey(), []byte(headId))
}

func (l *listStorage) AddRawRecord(ctx context.Context, rec *aclrecordproto.RawAclRecordWithId) error {
	return l.db.Put(l.keys.RawRecordKey(rec.Id), rec.Payload)
}

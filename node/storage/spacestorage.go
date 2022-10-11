package storage

import (
	"github.com/akrylysov/pogreb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	spacestorage "github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/gogo/protobuf/proto"
	"path"
	"sync"
)

type spaceStorage struct {
	objDb *pogreb.DB
	keys  spaceKeys
	mx    sync.Mutex
}

func newSpaceStorage(rootPath string, spaceId string) (store spacestorage.SpaceStorage, err error) {
	dbPath := path.Join(rootPath, spaceId)
	objDb, err := pogreb.Open(dbPath, nil)
	if err != nil {
		return
	}
	keys := spaceKeys{}
	has, err := objDb.Has([]byte(keys.HeaderKey()))
	if err != nil {
		return
	}
	if !has {
		err = spacestorage.ErrSpaceStorageMissing
		return
	}

	has, err = objDb.Has([]byte(keys.ACLKey()))
	if err != nil {
		return
	}
	if !has {
		err = spacestorage.ErrSpaceStorageMissing
		return
	}

	store = &spaceStorage{
		objDb: objDb,
		keys:  keys,
	}
	return
}

func createSpaceStorage(rootPath string, payload spacestorage.SpaceStorageCreatePayload) (store spacestorage.SpaceStorage, err error) {
	// TODO: add payload verification
	dbPath := path.Join(rootPath, payload.SpaceHeaderWithId.Id)
	db, err := pogreb.Open(dbPath, nil)
	if err != nil {
		return
	}

	keys := spaceKeys{}
	has, err := db.Has([]byte(keys.HeaderKey()))
	if err != nil {
		return
	}
	if has {
		err = spacestorage.ErrSpaceStorageExists
		return
	}

	marshalledRec, err := payload.RecWithId.Marshal()
	if err != nil {
		return
	}
	err = db.Put([]byte(keys.ACLKey()), marshalledRec)
	if err != nil {
		return
	}

	marshalledHeader, err := payload.SpaceHeaderWithId.Marshal()
	if err != nil {
		return
	}
	err = db.Put([]byte(keys.HeaderKey()), marshalledHeader)
	if err != nil {
		return
	}

	store = &spaceStorage{
		objDb: db,
		keys:  keys,
	}
	return
}

func (s *spaceStorage) TreeStorage(id string) (storage.TreeStorage, error) {
	return newTreeStorage(s.objDb, id)
}

func (s *spaceStorage) CreateTreeStorage(payload storage.TreeStorageCreatePayload) (ts storage.TreeStorage, err error) {
	s.mx.Lock()
	defer s.mx.Unlock()

	treeKeys := treeKeys{payload.TreeId}
	has, err := s.objDb.Has([]byte(treeKeys.RootKey()))
	if err != nil {
		return
	}
	if has {
		err = spacestorage.ErrSpaceStorageExists
		return
	}

	return createTreeStorage(s.objDb, payload)
}

func (s *spaceStorage) ACLStorage() (storage.ListStorage, error) {
	return nil, nil
}

func (s *spaceStorage) SpaceHeader() (header *spacesyncproto.RawSpaceHeaderWithId, err error) {
	res, err := s.objDb.Get([]byte(s.keys.HeaderKey()))
	if err != nil {
		return
	}

	header = &spacesyncproto.RawSpaceHeaderWithId{}
	err = proto.Unmarshal(res, header)
	return
}

func (s *spaceStorage) StoredIds() (ids []string, err error) {
	index := s.objDb.Items()

	_, value, err := index.Next()
	for err == nil {
		strVal := string(value)
		if isTreeKey(strVal) {
			ids = append(ids, string(value))
		}
		_, value, err = index.Next()
	}

	if err != pogreb.ErrIterationDone {
		return
	}
	err = nil
	return
}

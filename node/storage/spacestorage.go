package storage

import (
	"github.com/akrylysov/pogreb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	spacestorage "github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	storage2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
	"go.uber.org/zap"
	"path"
	"sync"
	"time"
)

var defPogrebOptions = &pogreb.Options{
	BackgroundCompactionInterval: time.Minute * 5,
}

var log = logger.NewNamed("storage.spacestorage")

type spaceStorage struct {
	spaceId    string
	objDb      *pogreb.DB
	keys       spaceKeys
	aclStorage storage2.ListStorage
	header     *spacesyncproto.RawSpaceHeaderWithId
	mx         sync.Mutex
}

func newSpaceStorage(rootPath string, spaceId string) (store spacestorage.SpaceStorage, err error) {
	log.With(zap.String("id", spaceId)).Debug("space storage opening with new")
	dbPath := path.Join(rootPath, spaceId)
	objDb, err := pogreb.Open(dbPath, defPogrebOptions)
	if err != nil {
		return
	}

	defer func() {
		if err != nil {
			log.With(zap.String("id", spaceId), zap.Error(err)).Warn("failed to open storage")
			objDb.Close()
		}
	}()

	keys := newSpaceKeys(spaceId)
	has, err := objDb.Has(keys.SpaceIdKey())
	if err != nil {
		return
	}
	if !has {
		err = spacestorage.ErrSpaceStorageMissing
		return
	}

	header, err := objDb.Get(keys.HeaderKey())
	if err != nil {
		return
	}
	if header == nil {
		err = spacestorage.ErrSpaceStorageMissing
		return
	}

	aclStorage, err := newListStorage(objDb)
	if err != nil {
		return
	}

	store = &spaceStorage{
		spaceId: spaceId,
		objDb:   objDb,
		keys:    keys,
		header: &spacesyncproto.RawSpaceHeaderWithId{
			RawHeader: header,
			Id:        spaceId,
		},
		aclStorage: aclStorage,
	}
	return
}

func createSpaceStorage(rootPath string, payload spacestorage.SpaceStorageCreatePayload) (store spacestorage.SpaceStorage, err error) {
	log.With(zap.String("id", payload.SpaceHeaderWithId.Id)).Debug("space storage creating")
	dbPath := path.Join(rootPath, payload.SpaceHeaderWithId.Id)
	db, err := pogreb.Open(dbPath, defPogrebOptions)
	if err != nil {
		return
	}

	defer func() {
		log.With(zap.String("id", payload.SpaceHeaderWithId.Id), zap.Error(err)).Warn("failed to create storage")
		if err != nil {
			db.Close()
		}
	}()

	keys := newSpaceKeys(payload.SpaceHeaderWithId.Id)
	has, err := db.Has(keys.SpaceIdKey())
	if err != nil {
		return
	}
	if has {
		err = spacesyncproto.ErrSpaceExists
		return
	}

	aclStorage, err := createListStorage(db, payload.RecWithId)
	if err != nil {
		return
	}

	err = db.Put(keys.HeaderKey(), payload.SpaceHeaderWithId.RawHeader)
	if err != nil {
		return
	}

	err = db.Put(keys.SpaceIdKey(), []byte(payload.SpaceHeaderWithId.Id))
	if err != nil {
		return
	}

	store = &spaceStorage{
		spaceId:    payload.SpaceHeaderWithId.Id,
		objDb:      db,
		keys:       keys,
		aclStorage: aclStorage,
		header:     payload.SpaceHeaderWithId,
	}
	return
}

func (s *spaceStorage) ID() (string, error) {
	return s.spaceId, nil
}

func (s *spaceStorage) TreeStorage(id string) (storage2.TreeStorage, error) {
	return newTreeStorage(s.objDb, id)
}

func (s *spaceStorage) CreateTreeStorage(payload storage2.TreeStorageCreatePayload) (ts storage2.TreeStorage, err error) {
	// we have mutex here, so we prevent overwriting the heads of a tree on concurrent creation
	s.mx.Lock()
	defer s.mx.Unlock()

	return createTreeStorage(s.objDb, payload)
}

func (s *spaceStorage) ACLStorage() (storage2.ListStorage, error) {
	return s.aclStorage, nil
}

func (s *spaceStorage) SpaceHeader() (header *spacesyncproto.RawSpaceHeaderWithId, err error) {
	return s.header, nil
}

func (s *spaceStorage) StoredIds() (ids []string, err error) {
	index := s.objDb.Items()

	key, val, err := index.Next()
	for err == nil {
		strKey := string(key)
		if isRootIdKey(strKey) {
			ids = append(ids, string(val))
		}
		key, val, err = index.Next()
	}

	if err != pogreb.ErrIterationDone {
		return
	}
	err = nil
	return
}

func (s *spaceStorage) Close() (err error) {
	log.With(zap.String("id", s.spaceId)).Debug("space storage closed")
	return s.objDb.Close()
}

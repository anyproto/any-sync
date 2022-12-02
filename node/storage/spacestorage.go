package storage

import (
	"github.com/akrylysov/pogreb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	spacestorage "github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
	"go.uber.org/zap"
	"path"
	"time"
)

var (
	defPogrebOptions    = &pogreb.Options{BackgroundCompactionInterval: time.Minute * 5}
	log                 = logger.NewNamed("storage.spacestorage")
	spaceValidationFunc = spacestorage.ValidateSpaceStorageCreatePayload
)

type spaceStorage struct {
	spaceId         string
	spaceSettingsId string
	objDb           *pogreb.DB
	keys            spaceKeys
	aclStorage      storage.ListStorage
	header          *spacesyncproto.RawSpaceHeaderWithId
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

	spaceSettingsId, err := objDb.Get(keys.SpaceSettingsIdKey())
	if err != nil {
		return
	}
	if spaceSettingsId == nil {
		err = spacestorage.ErrSpaceStorageMissing
		return
	}

	aclStorage, err := newListStorage(objDb)
	if err != nil {
		return
	}

	store = &spaceStorage{
		spaceId:         spaceId,
		spaceSettingsId: string(spaceSettingsId),
		objDb:           objDb,
		keys:            keys,
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
		if err != nil {
			log.With(zap.String("id", payload.SpaceHeaderWithId.Id), zap.Error(err)).Warn("failed to create storage")
			db.Close()
		}
	}()

	keys := newSpaceKeys(payload.SpaceHeaderWithId.Id)
	has, err := db.Has(keys.SpaceIdKey())
	if err != nil {
		return
	}
	if has {
		err = spacestorage.ErrSpaceStorageExists
		return
	}
	err = spaceValidationFunc(payload)
	if err != nil {
		return
	}

	aclStorage, err := createListStorage(db, payload.AclWithId)
	if err != nil {
		return
	}

	store = &spaceStorage{
		spaceId:         payload.SpaceHeaderWithId.Id,
		objDb:           db,
		keys:            keys,
		aclStorage:      aclStorage,
		spaceSettingsId: payload.SpaceSettingsWithId.Id,
		header:          payload.SpaceHeaderWithId,
	}

	_, err = store.CreateTreeStorage(storage.TreeStorageCreatePayload{
		RootRawChange: payload.SpaceSettingsWithId,
		Changes:       []*treechangeproto.RawTreeChangeWithId{payload.SpaceSettingsWithId},
		Heads:         []string{payload.SpaceSettingsWithId.Id},
	})
	if err != nil {
		return
	}

	err = db.Put(keys.SpaceSettingsIdKey(), []byte(payload.SpaceSettingsWithId.Id))
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

	return
}

func (s *spaceStorage) Id() string {
	return s.spaceId
}

func (s *spaceStorage) SpaceSettingsId() string {
	return s.spaceSettingsId
}

func (s *spaceStorage) TreeStorage(id string) (storage.TreeStorage, error) {
	return newTreeStorage(s.objDb, id)
}

func (s *spaceStorage) CreateTreeStorage(payload storage.TreeStorageCreatePayload) (ts storage.TreeStorage, err error) {
	return createTreeStorage(s.objDb, payload)
}

func (s *spaceStorage) ACLStorage() (storage.ListStorage, error) {
	return s.aclStorage, nil
}

func (s *spaceStorage) SpaceHeader() (header *spacesyncproto.RawSpaceHeaderWithId, err error) {
	return s.header, nil
}

func (s *spaceStorage) SetTreeDeletedStatus(id, state string) (err error) {
	return s.objDb.Put(s.keys.TreeDeletedKey(id), []byte(state))
}

func (s *spaceStorage) TreeDeletedStatus(id string) (status string, err error) {
	res, err := s.objDb.Get(s.keys.TreeDeletedKey(id))
	if err != nil {
		return
	}
	status = string(res)
	return
}

func (s *spaceStorage) StoredIds() (ids []string, err error) {
	index := s.objDb.Items()

	key, _, err := index.Next()
	for err == nil {
		strKey := string(key)
		if isTreeHeadsKey(strKey) {
			ids = append(ids, getRootId(strKey))
		}
		key, _, err = index.Next()
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

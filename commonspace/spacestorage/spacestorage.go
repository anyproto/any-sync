//go:generate mockgen -destination mock_spacestorage/mock_spacestorage.go github.com/anyproto/any-sync/commonspace/spacestorage SpaceStorage
package spacestorage

import (
	"context"
	"errors"

	anystore "github.com/anyproto/any-store"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/headsync/statestorage"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

const CName = "common.commonspace.spacestorage"

var (
	ErrSpaceStorageExists   = errors.New("space storage exists")
	ErrSpaceStorageMissing  = errors.New("space storage missing")
	ErrIncorrectSpaceHeader = errors.New("incorrect space header")

	ErrTreeStorageAlreadyDeleted = errors.New("tree storage already deleted")
)

type SpaceStorage interface {
	app.ComponentRunnable
	Id() string
	HeadStorage() headstorage.HeadStorage
	StateStorage() statestorage.StateStorage
	AclStorage() (list.Storage, error)
	TreeStorage(ctx context.Context, id string) (objecttree.Storage, error)
	CreateTreeStorage(ctx context.Context, payload treestorage.TreeStorageCreatePayload) (objecttree.Storage, error)
	AnyStore() anystore.DB
}

type SpaceStorageCreatePayload struct {
	AclWithId           *consensusproto.RawRecordWithId
	SpaceHeaderWithId   *spacesyncproto.RawSpaceHeaderWithId
	SpaceSettingsWithId *treechangeproto.RawTreeChangeWithId
}

type SpaceStorageProvider interface {
	app.Component
	WaitSpaceStorage(ctx context.Context, id string) (SpaceStorage, error)
	SpaceExists(id string) bool
	CreateSpaceStorage(ctx context.Context, payload SpaceStorageCreatePayload) (SpaceStorage, error)
}

func Create(ctx context.Context, store anystore.DB, payload SpaceStorageCreatePayload) (SpaceStorage, error) {
	spaceId := payload.SpaceHeaderWithId.Id
	state := statestorage.State{
		AclId:       payload.AclWithId.Id,
		SettingsId:  payload.SpaceSettingsWithId.Id,
		SpaceId:     payload.SpaceHeaderWithId.Id,
		SpaceHeader: payload.SpaceHeaderWithId.RawHeader,
	}
	changesColl, err := store.Collection(ctx, objecttree.CollName)
	if err != nil {
		return nil, err
	}
	orderIdx := anystore.IndexInfo{
		Fields: []string{objecttree.TreeKey, objecttree.OrderKey},
		Unique: true,
	}
	err = changesColl.EnsureIndex(ctx, orderIdx)
	if err != nil {
		return nil, err
	}
	// TODO: put it in one transaction
	stateStorage, err := statestorage.Create(ctx, state, store)
	if err != nil {
		return nil, err
	}
	headStorage, err := headstorage.New(ctx, store)
	if err != nil {
		return nil, err
	}
	aclStorage, err := list.CreateStorage(ctx, &consensusproto.RawRecordWithId{
		Payload: payload.AclWithId.Payload,
		Id:      payload.AclWithId.Id,
	}, headStorage, store)
	if err != nil {
		return nil, err
	}
	_, err = objecttree.CreateStorage(ctx, &treechangeproto.RawTreeChangeWithId{
		RawChange: payload.SpaceSettingsWithId.RawChange,
		Id:        payload.SpaceSettingsWithId.Id,
	}, headStorage, store)
	if err != nil {
		return nil, err
	}
	return &spaceStorage{
		store:        store,
		spaceId:      spaceId,
		headStorage:  headStorage,
		stateStorage: stateStorage,
		aclStorage:   aclStorage,
	}, nil
}

func New(ctx context.Context, spaceId string, store anystore.DB) (SpaceStorage, error) {
	s := &spaceStorage{
		store:   store,
		spaceId: spaceId,
	}
	changesColl, err := store.OpenCollection(ctx, objecttree.CollName)
	if err != nil {
		return nil, err
	}
	orderIdx := anystore.IndexInfo{
		Fields: []string{objecttree.TreeKey, objecttree.OrderKey},
		Unique: true,
	}
	err = changesColl.EnsureIndex(ctx, orderIdx)
	if err != nil {
		return nil, err
	}
	s.headStorage, err = headstorage.New(ctx, s.store)
	if err != nil {
		return nil, err
	}
	s.stateStorage, err = statestorage.New(ctx, s.spaceId, s.store)
	if err != nil {
		return nil, err
	}
	state, err := s.stateStorage.GetState(ctx)
	if err != nil {
		return nil, err
	}
	s.aclStorage, err = list.NewStorage(ctx, state.AclId, s.headStorage, s.store)
	if err != nil {
		return nil, err
	}
	return s, nil
}

type spaceStorage struct {
	spaceId      string
	headStorage  headstorage.HeadStorage
	stateStorage statestorage.StateStorage
	aclStorage   list.Storage
	store        anystore.DB
}

func (s *spaceStorage) Run(ctx context.Context) (err error) {
	return nil
}

func (s *spaceStorage) Close(ctx context.Context) (err error) {
	return nil
}

func (s *spaceStorage) Name() (name string) {
	return CName
}

func (s *spaceStorage) Id() string {
	return s.spaceId
}

func (s *spaceStorage) AnyStore() anystore.DB {
	return s.store
}

func (s *spaceStorage) HeadStorage() headstorage.HeadStorage {
	return s.headStorage
}

func (s *spaceStorage) StateStorage() statestorage.StateStorage {
	return s.stateStorage
}

func (s *spaceStorage) AclStorage() (list.Storage, error) {
	return s.aclStorage, nil
}

func (s *spaceStorage) TreeStorage(ctx context.Context, id string) (objecttree.Storage, error) {
	return objecttree.NewStorage(ctx, id, s.headStorage, s.store)
}

func (s *spaceStorage) CreateTreeStorage(ctx context.Context, payload treestorage.TreeStorageCreatePayload) (objecttree.Storage, error) {
	return objecttree.CreateStorage(ctx, payload.RootRawChange, s.headStorage, s.store)
}

func (s *spaceStorage) Init(a *app.App) (err error) {
	return nil
}

//go:generate mockgen -destination mock_spacestorage/mock_spacestorage.go github.com/anyproto/any-sync/commonspace/spacestorage SpaceStorage
package spacestorage

import (
	"context"
	"errors"
	"sync/atomic"

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
	CreateStorageWithDeferredCreation(ctx context.Context, payload treestorage.TreeStorageCreatePayload) (objecttree.Storage, error)
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

func Create(ctx context.Context, store anystore.DB, payload SpaceStorageCreatePayload) (st SpaceStorage, err error) {
	spaceId := payload.SpaceHeaderWithId.Id
	state := statestorage.State{
		AclId:       payload.AclWithId.Id,
		SettingsId:  payload.SpaceSettingsWithId.Id,
		SpaceId:     payload.SpaceHeaderWithId.Id,
		SpaceHeader: payload.SpaceHeaderWithId.RawHeader,
	}
	tx, err := store.WriteTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()
	changesColl, err := store.Collection(tx.Context(), objecttree.CollName)
	if err != nil {
		return nil, err
	}
	orderIdx := anystore.IndexInfo{
		Fields: []string{objecttree.TreeKey, objecttree.OrderKey},
		Unique: true,
	}
	err = changesColl.EnsureIndex(tx.Context(), orderIdx)
	if err != nil {
		return nil, err
	}
	// TODO: put it in one transaction
	stateStorage, err := statestorage.CreateTx(tx.Context(), state, store)
	if err != nil {
		if errors.Is(err, anystore.ErrDocExists) {
			return nil, ErrSpaceStorageExists
		}
		return nil, err
	}
	headStorage, err := headstorage.New(tx.Context(), store)
	if err != nil {
		return nil, err
	}
	aclStorage, err := list.CreateStorageTx(tx.Context(), &consensusproto.RawRecordWithId{
		Payload: payload.AclWithId.Payload,
		Id:      payload.AclWithId.Id,
	}, headStorage, store)
	if err != nil {
		return nil, err
	}
	s := &spaceStorage{
		store:        store,
		spaceId:      spaceId,
		headStorage:  headStorage,
		stateStorage: stateStorage,
		aclStorage:   aclStorage,
	}
	_, err = objecttree.CreateStorageTx(tx.Context(), &treechangeproto.RawTreeChangeWithId{
		RawChange: payload.SpaceSettingsWithId.RawChange,
		Id:        payload.SpaceSettingsWithId.Id,
	}, headStorage, store)
	if err != nil {
		return nil, err
	}
	return s, nil
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
	// Initialize in-memory addSeq counter from max LastAddSeq in heads collection
	if maxSeq, err := s.headStorage.MaxLastAddSeq(ctx); err == nil {
		s.addSeq.Store(maxSeq)
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
	addSeq       atomic.Uint64 // space-global apply sequence counter (in-memory)
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

// addSeqSetter is implemented by storage types that support space-global apply sequences.
type addSeqSetter interface {
	SetAddSeq(seq *atomic.Uint64)
}

func (s *spaceStorage) setAddSeq(st objecttree.Storage) {
	if setter, ok := st.(addSeqSetter); ok {
		setter.SetAddSeq(&s.addSeq)
	}
}

func (s *spaceStorage) TreeStorage(ctx context.Context, id string) (objecttree.Storage, error) {
	st, err := objecttree.NewStorage(ctx, id, s.headStorage, s.store)
	if err != nil {
		return nil, err
	}
	s.setAddSeq(st)
	return st, nil
}

func (s *spaceStorage) CreateTreeStorage(ctx context.Context, payload treestorage.TreeStorageCreatePayload) (objecttree.Storage, error) {
	st, err := objecttree.CreateStorage(ctx, payload.RootRawChange, s.headStorage, s.store)
	if err != nil {
		return nil, err
	}
	s.setAddSeq(st)
	return st, nil
}

func (s *spaceStorage) CreateStorageWithDeferredCreation(ctx context.Context, payload treestorage.TreeStorageCreatePayload) (objecttree.Storage, error) {
	st, err := objecttree.CreateStorageWithDeferredCreation(ctx, payload.RootRawChange, s.headStorage, s.store)
	if err != nil {
		return nil, err
	}
	s.setAddSeq(st)
	return st, nil
}

func (s *spaceStorage) Init(a *app.App) (err error) {
	return nil
}

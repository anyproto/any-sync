//go:generate mockgen -destination mock_statestorage/mock_statestorage.go github.com/anyproto/any-sync/commonspace/headsync/statestorage StateStorage
package statestorage

import (
	anystore "github.com/anyproto/any-store"
	"github.com/anyproto/any-store/anyenc"
	"github.com/anyproto/any-store/query"
	"golang.org/x/net/context"
)

type State struct {
	OldHash     string
	NewHash     string
	AclId       string
	SettingsId  string
	SpaceId     string
	SpaceHeader []byte
}

type Observer interface {
	OnHashChange(oldHash, newHash string)
}

type StateStorage interface {
	GetState(ctx context.Context) (State, error)
	SettingsId() string
	SetHash(ctx context.Context, oldHash, newHash string) error
	SetObserver(observer Observer)
}

const (
	stateCollectionKey = "state"
	idKey              = "id"
	oldHashKey         = "oh"
	newHashKey         = "nh"
	legacyHashKey      = "h"
	headerKey          = "e"
	aclIdKey           = "a"
	settingsIdKey      = "s"
)

type stateStorage struct {
	spaceId    string
	settingsId string
	aclId      string
	observer   Observer
	store      anystore.DB
	stateColl  anystore.Collection
	arena      *anyenc.Arena
}

func (s *stateStorage) GetState(ctx context.Context) (State, error) {
	doc, err := s.stateColl.FindId(ctx, s.spaceId)
	if err != nil {
		return State{}, err
	}
	return s.stateFromDoc(doc), nil
}

func (s *stateStorage) SetObserver(observer Observer) {
	s.observer = observer
}

func (s *stateStorage) SetHash(ctx context.Context, oldHash, newHash string) (err error) {
	defer func() {
		if s.observer != nil && err == nil {
			s.observer.OnHashChange(oldHash, newHash)
		}
	}()
	tx, err := s.stateColl.WriteTx(ctx)
	if err != nil {
		return err
	}
	mod := query.ModifyFunc(func(a *anyenc.Arena, v *anyenc.Value) (result *anyenc.Value, modified bool, err error) {
		v.Set(oldHashKey, a.NewString(oldHash))
		v.Set(newHashKey, a.NewString(newHash))
		return v, true, nil
	})
	_, err = s.stateColl.UpsertId(tx.Context(), s.spaceId, mod)
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func New(ctx context.Context, spaceId string, store anystore.DB) (StateStorage, error) {
	stateCollection, err := store.Collection(ctx, stateCollectionKey)
	if err != nil {
		return nil, err
	}
	storage := &stateStorage{
		store:     store,
		spaceId:   spaceId,
		stateColl: stateCollection,
		arena:     &anyenc.Arena{},
	}
	st, err := storage.GetState(ctx)
	if err != nil {
		return nil, err
	}
	storage.settingsId = st.SettingsId
	return storage, nil
}

func Create(ctx context.Context, state State, store anystore.DB) (st StateStorage, err error) {
	tx, err := store.WriteTx(ctx)
	if err != nil {
		return nil, err
	}
	storage, err := CreateTx(tx.Context(), state, store)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	return storage, tx.Commit()
}

func CreateTx(ctx context.Context, state State, store anystore.DB) (StateStorage, error) {
	arena := &anyenc.Arena{}
	stateCollection, err := store.Collection(ctx, stateCollectionKey)
	if err != nil {
		return nil, err
	}
	defer arena.Reset()
	doc := arena.NewObject()
	doc.Set(idKey, arena.NewString(state.SpaceId))
	doc.Set(settingsIdKey, arena.NewString(state.SettingsId))
	doc.Set(headerKey, arena.NewBinary(state.SpaceHeader))
	doc.Set(aclIdKey, arena.NewString(state.AclId))
	err = stateCollection.Insert(ctx, doc)
	if err != nil {
		return nil, err
	}
	return &stateStorage{
		spaceId:    state.SpaceId,
		store:      store,
		settingsId: state.SettingsId,
		stateColl:  stateCollection,
		arena:      arena,
	}, nil
}

func (s *stateStorage) SettingsId() string {
	return s.settingsId
}

func (s *stateStorage) stateFromDoc(doc anystore.Doc) State {
	var (
		oldHash = doc.Value().GetString(oldHashKey)
		newHash = doc.Value().GetString(newHashKey)
	)
	// legacy hash is used for backward compatibility, which was due to a mistake in key names
	if oldHash == "" || newHash == "" {
		oldHash = doc.Value().GetString(legacyHashKey)
		newHash = oldHash
	}
	return State{
		SpaceId:     doc.Value().GetString(idKey),
		SettingsId:  doc.Value().GetString(settingsIdKey),
		AclId:       doc.Value().GetString(aclIdKey),
		OldHash:     oldHash,
		NewHash:     newHash,
		SpaceHeader: doc.Value().GetBytes(headerKey),
	}
}

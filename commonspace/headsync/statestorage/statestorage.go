package statestorage

import (
	anystore "github.com/anyproto/any-store"
	"github.com/anyproto/any-store/anyenc"
	"github.com/anyproto/any-store/query"
	"golang.org/x/net/context"
)

type State struct {
	Hash       string
	AclId      string
	SettingsId string
	SpaceId    string
}

type StateStorage interface {
	GetState(ctx context.Context) (State, error)
	SetHash(ctx context.Context, hash string) error
}

const (
	stateCollectionKey = "state"
	idKey              = "id"
	hashKey            = "h"
	aclIdKey           = "a"
	settingsIdKey      = "s"
)

type stateStorage struct {
	spaceId   string
	store     anystore.DB
	stateColl anystore.Collection
	arena     *anyenc.Arena
}

func (s *stateStorage) GetState(ctx context.Context) (State, error) {
	doc, err := s.stateColl.FindId(ctx, s.spaceId)
	if err != nil {
		return State{}, err
	}
	return s.stateFromDoc(doc), nil
}

func (s *stateStorage) SetHash(ctx context.Context, hash string) error {
	mod := query.ModifyFunc(func(a *anyenc.Arena, v *anyenc.Value) (result *anyenc.Value, modified bool, err error) {
		v.Set(hashKey, a.NewString(hash))
		return v, true, nil
	})
	_, err := s.stateColl.UpsertId(ctx, s.spaceId, mod)
	return err
}

func New(ctx context.Context, spaceId string, store anystore.DB) (StateStorage, error) {
	stateCollection, err := store.Collection(ctx, stateCollectionKey)
	if err != nil {
		return nil, err
	}
	return &stateStorage{
		store:     store,
		spaceId:   spaceId,
		stateColl: stateCollection,
		arena:     &anyenc.Arena{},
	}, nil
}

func Create(ctx context.Context, state State, store anystore.DB) (StateStorage, error) {
	arena := &anyenc.Arena{}
	stateCollection, err := store.Collection(ctx, stateCollectionKey)
	if err != nil {
		return nil, err
	}
	tx, err := stateCollection.WriteTx(ctx)
	if err != nil {
		return nil, err
	}
	doc := arena.NewObject()
	doc.Set(idKey, arena.NewString(state.SpaceId))
	doc.Set(settingsIdKey, arena.NewString(state.SettingsId))
	doc.Set(aclIdKey, arena.NewString(state.AclId))
	err = stateCollection.Insert(tx.Context(), doc)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	return &stateStorage{
		spaceId:   state.SpaceId,
		store:     store,
		stateColl: stateCollection,
		arena:     &anyenc.Arena{},
	}, tx.Commit()
}

func (s *stateStorage) stateFromDoc(doc anystore.Doc) State {
	return State{
		SpaceId:    doc.Value().GetString(idKey),
		SettingsId: doc.Value().GetString(settingsIdKey),
		AclId:      doc.Value().GetString(aclIdKey),
		Hash:       doc.Value().GetString(hashKey),
	}
}

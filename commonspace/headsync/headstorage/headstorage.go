package headstorage

import (
	"fmt"

	anystore "github.com/anyproto/any-store"
	"github.com/anyproto/any-store/anyenc"
	"github.com/anyproto/any-store/query"
	"golang.org/x/net/context"

	"github.com/anyproto/any-sync/util/storeutil"
)

const (
	headsKey            = "h"
	commonSnapshotKey   = "s"
	idKey               = "id"
	deletedStatusKey    = "d"
	headsCollectionName = "heads"
)

type HeadsEntry struct {
	Id             string
	Heads          []string
	CommonSnapshot string
	DeletedStatus  string
}

type HeadsUpdate struct {
	Id             string
	Heads          []string
	CommonSnapshot *string
	DeletedStatus  *string
}

type EntryIterator func(entry HeadsEntry) (bool, error)

type HeadStorage interface {
	IterateEntries(ctx context.Context, iter EntryIterator) error
	GetEntry(ctx context.Context, id string) (HeadsEntry, error)
	UpdateEntryTx(txCtx context.Context, update HeadsUpdate) error
}

type headStorage struct {
	store     anystore.DB
	headsColl anystore.Collection
	arena     *anyenc.Arena
}

func New(ctx context.Context, store anystore.DB) (HeadStorage, error) {
	headsColl, err := store.Collection(ctx, headsCollectionName)
	if err != nil {
		return nil, err
	}
	st := &headStorage{
		store:     store,
		headsColl: headsColl,
		arena:     &anyenc.Arena{},
	}
	return st, nil
}

func (h *headStorage) IterateEntries(ctx context.Context, entryIter EntryIterator) error {
	iter, err := h.headsColl.Find(nil).Sort(idKey).Iter(ctx)
	if err != nil {
		return fmt.Errorf("find iter: %w", err)
	}
	defer iter.Close()

	for iter.Next() {
		doc, err := iter.Doc()
		if err != nil {
			return fmt.Errorf("doc not found: %w", err)
		}
		cont, err := entryIter(h.entryFromDoc(doc))
		if !cont {
			return err
		}
	}
	return nil
}

func (h *headStorage) GetEntry(ctx context.Context, id string) (HeadsEntry, error) {
	doc, err := h.headsColl.FindId(ctx, id)
	if err != nil {
		return HeadsEntry{}, err
	}
	return h.entryFromDoc(doc), nil
}

func (h *headStorage) UpdateEntryTx(ctx context.Context, update HeadsUpdate) (err error) {
	mod := query.ModifyFunc(func(a *anyenc.Arena, v *anyenc.Value) (result *anyenc.Value, modified bool, err error) {
		if update.DeletedStatus != nil {
			v.Set(deletedStatusKey, a.NewString(*update.DeletedStatus))
		}
		if update.CommonSnapshot != nil {
			v.Set(commonSnapshotKey, a.NewString(*update.CommonSnapshot))
		}
		if update.Heads != nil {
			v.Set(headsKey, storeutil.NewStringArrayValue(update.Heads, a))
		}
		return v, true, nil
	})
	_, err = h.headsColl.UpsertId(ctx, update.Id, mod)
	return
}

func (h *headStorage) entryFromDoc(doc anystore.Doc) HeadsEntry {
	return HeadsEntry{
		Id:             doc.Value().GetString(idKey),
		Heads:          storeutil.StringsFromArrayValue(doc.Value(), headsKey),
		CommonSnapshot: doc.Value().GetString(commonSnapshotKey),
		DeletedStatus:  doc.Value().GetString(deletedStatusKey),
	}
}

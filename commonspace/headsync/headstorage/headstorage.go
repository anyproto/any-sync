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
	derivedStatusKey    = "r"
	headsCollectionName = "heads"
)

type DeletedStatus int

const (
	DeletedStatusNotDeleted DeletedStatus = iota
	DeletedStatusQueued
	DeletedStatusDeleted
)

type HeadsEntry struct {
	Id             string
	Heads          []string
	CommonSnapshot string
	DeletedStatus  DeletedStatus
	IsDerived      bool
}

type HeadsUpdate struct {
	Id             string
	Heads          []string
	CommonSnapshot *string
	DeletedStatus  *DeletedStatus
	IsDerived      *bool
}

type EntryIterator func(entry HeadsEntry) (bool, error)

type IterOpts struct {
	Deleted bool
}

type HeadStorage interface {
	AddObserver(observer Observer)
	IterateEntries(ctx context.Context, iterOpts IterOpts, iter EntryIterator) error
	GetEntry(ctx context.Context, id string) (HeadsEntry, error)
	DeleteEntryTx(txCtx context.Context, id string) error
	UpdateEntryTx(txCtx context.Context, update HeadsUpdate) error
	UpdateEntry(ctx context.Context, update HeadsUpdate) error
}

type Observer interface {
	OnUpdate(update HeadsUpdate)
}

type headStorage struct {
	store     anystore.DB
	headsColl anystore.Collection
	observers []Observer
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
	deletedIdx := anystore.IndexInfo{
		Name:   deletedStatusKey,
		Fields: []string{deletedStatusKey},
		Sparse: true,
	}
	return st, st.headsColl.EnsureIndex(ctx, deletedIdx)
}

func (h *headStorage) AddObserver(observer Observer) {
	// we don't take lock here, because it shouldn't happen during the program execution
	h.observers = append(h.observers, observer)
}

func (h *headStorage) IterateEntries(ctx context.Context, opts IterOpts, entryIter EntryIterator) error {
	var qry any
	if opts.Deleted {
		qry = query.Key{Path: []string{deletedStatusKey}, Filter: query.NewComp(query.CompOpGte, int(DeletedStatusQueued))}
	} else {
		qry = query.Key{Path: []string{deletedStatusKey}, Filter: query.Not{query.Exists{}}}
	}
	iter, err := h.headsColl.Find(qry).Sort(idKey).Iter(ctx)
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

func (h *headStorage) UpdateEntry(ctx context.Context, update HeadsUpdate) (err error) {
	tx, err := h.headsColl.WriteTx(ctx)
	if err != nil {
		return
	}
	err = h.UpdateEntryTx(tx.Context(), update)
	if err != nil {
		tx.Rollback()
		return
	}
	return tx.Commit()
}

func (h *headStorage) UpdateEntryTx(ctx context.Context, update HeadsUpdate) (err error) {
	defer func() {
		if err == nil {
			for _, observer := range h.observers {
				observer.OnUpdate(update)
			}
		}
	}()
	mod := query.ModifyFunc(func(a *anyenc.Arena, v *anyenc.Value) (result *anyenc.Value, modified bool, err error) {
		if update.DeletedStatus != nil {
			v.Set(deletedStatusKey, a.NewNumberInt(int(*update.DeletedStatus)))
		}
		if update.CommonSnapshot != nil {
			v.Set(commonSnapshotKey, a.NewString(*update.CommonSnapshot))
		}
		if update.Heads != nil {
			v.Set(headsKey, storeutil.NewStringArrayValue(update.Heads, a))
		}
		if update.IsDerived != nil {
			if *update.IsDerived {
				v.Set(derivedStatusKey, a.NewTrue())
			} else {
				v.Set(derivedStatusKey, a.NewFalse())
			}
		}
		return v, true, nil
	})
	_, err = h.headsColl.UpsertId(ctx, update.Id, mod)
	return
}

func (h *headStorage) DeleteEntryTx(ctx context.Context, id string) error {
	return h.headsColl.DeleteId(ctx, id)
}

func (h *headStorage) entryFromDoc(doc anystore.Doc) HeadsEntry {
	return HeadsEntry{
		Id:             doc.Value().GetString(idKey),
		Heads:          storeutil.StringsFromArrayValue(doc.Value(), headsKey),
		CommonSnapshot: doc.Value().GetString(commonSnapshotKey),
		DeletedStatus:  DeletedStatus(doc.Value().GetInt(deletedStatusKey)),
		IsDerived:      doc.Value().GetBool(derivedStatusKey),
	}
}

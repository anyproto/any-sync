//go:generate mockgen -destination mock_headstorage/mock_headstorage.go github.com/anyproto/any-sync/commonspace/headsync/headstorage HeadStorage
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
	DeletedStatusKey    = "d"
	derivedStatusKey    = "r"
	HeadsCollectionName = "heads"
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
	DeleteEntry(ctx context.Context, id string) error
	UpdateEntry(ctx context.Context, update HeadsUpdate) error
}

type Observer interface {
	OnUpdate(update HeadsEntry)
}

type headStorage struct {
	store      anystore.DB
	headsColl  anystore.Collection
	observers  []Observer
	parserPool *anyenc.ParserPool
}

func New(ctx context.Context, store anystore.DB) (HeadStorage, error) {
	headsColl, err := store.Collection(ctx, HeadsCollectionName)
	if err != nil {
		return nil, err
	}
	st := &headStorage{
		store:      store,
		headsColl:  headsColl,
		parserPool: &anyenc.ParserPool{},
	}
	deletedIdx := anystore.IndexInfo{
		Name:   DeletedStatusKey,
		Fields: []string{DeletedStatusKey},
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
		qry = query.Key{Path: []string{DeletedStatusKey}, Filter: query.NewComp(query.CompOpGte, int(DeletedStatusQueued))}
	} else {
		qry = query.Key{Path: []string{DeletedStatusKey}, Filter: query.Not{query.Exists{}}}
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
		cont, err := entryIter(entryFromVal(doc.Value()))
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
	return entryFromVal(doc.Value()), nil
}

func (h *headStorage) UpdateEntry(ctx context.Context, update HeadsUpdate) (err error) {
	var (
		resultEntry HeadsEntry
		modResult   anystore.ModifyResult
	)
	defer func() {
		if err == nil && modResult.Modified > 0 {
			for _, observer := range h.observers {
				observer.OnUpdate(resultEntry)
			}
		}
	}()

	mod := query.ModifyFunc(func(a *anyenc.Arena, v *anyenc.Value) (result *anyenc.Value, modified bool, err error) {
		if update.DeletedStatus != nil {
			if storeutil.ModifyKey(v, DeletedStatusKey, a.NewNumberInt(int(*update.DeletedStatus))) {
				modified = true
			}
		}

		if update.CommonSnapshot != nil {
			if storeutil.ModifyKey(v, commonSnapshotKey, a.NewString(*update.CommonSnapshot)) {
				modified = true
			}
		}

		if update.Heads != nil {
			if storeutil.ModifyKey(v, headsKey, storeutil.NewStringArrayValue(update.Heads, a)) {
				modified = true
			}
		}

		if update.IsDerived != nil {
			var val *anyenc.Value
			if *update.IsDerived {
				val = a.NewTrue()
			} else {
				val = a.NewFalse()
			}
			if storeutil.ModifyKey(v, derivedStatusKey, val) {
				modified = true
			}
		}

		resultEntry = entryFromVal(v)
		return v, modified, nil
	})
	modResult, err = h.headsColl.UpsertId(ctx, update.Id, mod)
	return
}

func (h *headStorage) DeleteEntry(ctx context.Context, id string) error {
	return h.headsColl.DeleteId(ctx, id)
}

func entryFromVal(val *anyenc.Value) HeadsEntry {
	return HeadsEntry{
		Id:             val.GetString(idKey),
		Heads:          storeutil.StringsFromArrayValue(val, headsKey),
		CommonSnapshot: val.GetString(commonSnapshotKey),
		DeletedStatus:  DeletedStatus(val.GetInt(DeletedStatusKey)),
		IsDerived:      val.GetBool(derivedStatusKey),
	}
}

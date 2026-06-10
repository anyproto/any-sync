package innerstorage

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"strings"

	anystore "github.com/anyproto/any-store"
	"github.com/anyproto/any-store/anyenc"
	"github.com/anyproto/any-store/query"

	"github.com/anyproto/any-sync/app/ldiff"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
)

var (
	parserPool = &anyenc.ParserPool{}
	arenaPool  = &anyenc.ArenaPool{}
)

type KeyValueStorage interface {
	Set(ctx context.Context, keyValues ...KeyValue) (err error)
	Diff() ldiff.CompareDiff
	GetKeyPeerId(ctx context.Context, keyPeerId string) (keyValue KeyValue, err error)
	IterateValues(context.Context, func(kv KeyValue) (bool, error)) (err error)
	IteratePrefix(context.Context, string, func(kv KeyValue) error) (err error)
}

type storage struct {
	diff        ldiff.CompareDiff
	headStorage headstorage.HeadStorage
	collection  anystore.Collection
	store       anystore.DB
	storageName string
}

func New(ctx context.Context, storageName string, headStorage headstorage.HeadStorage, store anystore.DB) (kv KeyValueStorage, err error) {
	collection, err := store.Collection(ctx, storageName)
	if err != nil {
		return nil, err
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
	storage := &storage{
		storageName: storageName,
		headStorage: headStorage,
		collection:  collection,
		store:       store,
		diff:        ldiff.New(32, 256).(ldiff.CompareDiff),
	}
	iter, err := storage.collection.Find(nil).Iter(ctx)
	if err != nil {
		return
	}
	defer func() {
		_ = iter.Close()
	}()
	var (
		doc      anystore.Doc
		elements []ldiff.Element
	)
	for iter.Next() {
		if doc, err = iter.Doc(); err != nil {
			return
		}
		elements = append(elements, anyEncToElement(doc.Value()))
	}
	storage.diff.Set(elements...)
	hash := storage.diff.Hash()
	err = headStorage.UpdateEntry(tx.Context(), headstorage.HeadsUpdate{
		Id:    storageName,
		Heads: []string{hash},
	})
	return storage, err
}

func (s *storage) Diff() ldiff.CompareDiff {
	return s.diff
}

func (s *storage) GetKeyPeerId(ctx context.Context, keyPeerId string) (value KeyValue, err error) {
	doc, err := s.collection.FindId(ctx, keyPeerId)
	if err != nil {
		return
	}
	return s.keyValueFromDoc(doc), nil
}

func (s *storage) IterateValues(ctx context.Context, iterFunc func(kv KeyValue) (bool, error)) (err error) {
	iter, err := s.collection.Find(nil).Iter(ctx)
	if err != nil {
		return
	}
	defer func() {
		_ = iter.Close()
	}()
	var doc anystore.Doc
	for iter.Next() {
		if doc, err = iter.Doc(); err != nil {
			return
		}
		continueIteration, err := iterFunc(s.keyValueFromDoc(doc))
		if err != nil {
			return err
		}
		if !continueIteration {
			break
		}
	}
	return nil
}

func (s *storage) IteratePrefix(ctx context.Context, prefix string, iterFunc func(kv KeyValue) error) (err error) {
	filter := query.Key{Path: []string{"id"}, Filter: query.NewComp(query.CompOpGte, prefix)}
	qry := s.collection.Find(filter).Sort("id")
	iter, err := qry.Iter(ctx)
	if err != nil {
		return
	}
	defer func() {
		_ = iter.Close()
	}()
	var doc anystore.Doc
	for iter.Next() {
		if doc, err = iter.Doc(); err != nil {
			return
		}
		if !strings.Contains(doc.Value().GetString("id"), prefix) {
			break
		}
		err := iterFunc(s.keyValueFromDoc(doc))
		if err != nil {
			return err
		}
	}
	return nil
}

// keyValueFromDoc returns a KeyValue that owns its memory. The byte slices are
// cloned because GetBytes aliases the doc's parse buffer, which iterator-based
// callers reuse for the next document (strings are safe: GetString copies).
func (s *storage) keyValueFromDoc(doc anystore.Doc) KeyValue {
	valueObj := doc.Value().GetObject("v")
	value := Value{
		Value:             bytes.Clone(valueObj.Get("v").GetBytes()),
		PeerSignature:     bytes.Clone(valueObj.Get("p").GetBytes()),
		IdentitySignature: bytes.Clone(valueObj.Get("i").GetBytes()),
	}
	return KeyValue{
		KeyPeerId:      doc.Value().GetString("id"),
		ReadKeyId:      doc.Value().GetString("r"),
		Value:          value,
		TimestampMicro: int64(doc.Value().GetFloat64("t")),
		Identity:       doc.Value().GetString("i"),
		PeerId:         doc.Value().GetString("p"),
		Key:            doc.Value().GetString("k"),
	}
}

func (s *storage) init(ctx context.Context) (err error) {
	s.diff = ldiff.New(32, 256).(ldiff.CompareDiff)
	iter, err := s.collection.Find(nil).Iter(ctx)
	if err != nil {
		return
	}
	defer func() {
		_ = iter.Close()
	}()
	var doc anystore.Doc
	var elements []ldiff.Element
	for iter.Next() {
		if doc, err = iter.Doc(); err != nil {
			return
		}
		elements = append(elements, anyEncToElement(doc.Value()))
	}
	s.diff.Set(elements...)
	return
}

func (s *storage) Set(ctx context.Context, values ...KeyValue) (err error) {
	tx, err := s.collection.WriteTx(ctx)
	if err != nil {
		return
	}
	var (
		elements, prior []ldiff.Element
		added           []string
		diffUpdated     bool
	)
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
		if err != nil && diffUpdated {
			// The diff already contains this call's elements but the tx did not
			// commit: undo the in-memory mutations so the diff never advertises
			// heads the storage doesn't hold — peers would never re-send those
			// values. Prior heads are restored in reverse so a duplicate id ends
			// up at its genuine pre-call state; inserts are then dropped.
			for i := len(prior) - 1; i >= 0; i-- {
				s.diff.Set(prior[i])
			}
			for _, id := range added {
				_ = s.diff.RemoveId(id)
			}
		}
	}()
	ctx = tx.Context()
	elements, prior, added, err = s.updateValues(ctx, values...)
	if err != nil {
		return
	}
	s.diff.Set(elements...)
	diffUpdated = len(elements) > 0
	err = s.headStorage.UpdateEntry(ctx, headstorage.HeadsUpdate{
		Id:    s.storageName,
		Heads: []string{s.diff.Hash()},
	})
	return
}

// updateValues upserts the values that win their LWW comparison and returns the
// new diff elements along with what is needed to undo the diff on a failed tx:
// the replaced elements' prior state and the ids that were not present before.
func (s *storage) updateValues(ctx context.Context, values ...KeyValue) (elements, prior []ldiff.Element, added []string, err error) {
	parser := parserPool.Get()
	defer parserPool.Put(parser)
	arena := arenaPool.Get()
	defer arenaPool.Put(arena)

	elements = make([]ldiff.Element, 0, len(values))
	var doc anystore.Doc
	for _, value := range values {
		doc, err = s.collection.FindIdWithParser(ctx, parser, value.KeyPeerId)
		isNotFound := errors.Is(err, anystore.ErrDocNotFound)
		if err != nil && !isNotFound {
			return
		}
		if !isNotFound {
			if int64(doc.Value().GetFloat64("t")) >= value.TimestampMicro {
				continue
			}
			prior = append(prior, anyEncToElement(doc.Value()))
		} else {
			added = append(added, value.KeyPeerId)
		}
		arena.Reset()
		val := value.AnyEnc(arena)
		if err = s.collection.UpsertOne(ctx, val); err != nil {
			return
		}
		elements = append(elements, anyEncToElement(val))
	}
	return
}

func anyEncToElement(val *anyenc.Value) ldiff.Element {
	byteRepr := make([]byte, 8)
	binary.BigEndian.PutUint64(byteRepr, uint64(int64(val.GetFloat64("t"))))
	return ldiff.Element{
		Id:   val.GetString("id"),
		Head: string(byteRepr),
	}
}

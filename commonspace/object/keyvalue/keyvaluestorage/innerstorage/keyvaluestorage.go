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
	// watermarks maps a deletion prefix to the highest applied watermark
	// timestamp: rows under the prefix older than it are dropped and stay
	// rejected. Mutated only inside Set, which callers serialize (the outer
	// storage holds its mutex around every write).
	watermarks map[string]int64
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
		watermarks:  map[string]int64{},
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
		if dp := doc.Value().GetString("dp"); dp != "" {
			if t := int64(doc.Value().GetFloat64("t")); t > storage.watermarks[dp] {
				storage.watermarks[dp] = t
			}
		}
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
	// Sorted by id (key+"-"+peerId) so all rows of one key come out
	// adjacent. Storage.Iterate builds its per-key callback groups from
	// consecutive runs; an unsorted scan is insertion-ordered and can
	// split a key across groups whenever peers' row batches interleave.
	iter, err := s.collection.Find(nil).Sort("id").Iter(ctx)
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
		DeletePrefix:   doc.Value().GetString("dp"),
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
		res         updateResult
		diffUpdated bool
	)
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
		if err == nil {
			for prefix, t := range res.watermarks {
				if t > s.watermarks[prefix] {
					s.watermarks[prefix] = t
				}
			}
		}
		if err != nil && diffUpdated {
			// The diff already contains this call's elements but the tx did not
			// commit: undo the in-memory mutations so the diff never advertises
			// heads the storage doesn't hold — peers would never re-send those
			// values. Prior heads are restored in reverse so a duplicate id ends
			// up at its genuine pre-call state; inserts are then dropped, and
			// watermark-dropped rows re-added (the tx rollback restored their
			// documents).
			for i := len(res.prior) - 1; i >= 0; i-- {
				s.diff.Set(res.prior[i])
			}
			for _, id := range res.added {
				_ = s.diff.RemoveId(id)
			}
			for _, el := range res.removed {
				s.diff.Set(el)
			}
		}
	}()
	ctx = tx.Context()
	res, err = s.updateValues(ctx, values...)
	if err != nil {
		return
	}
	s.diff.Set(res.elements...)
	for _, el := range res.removed {
		_ = s.diff.RemoveId(el.Id)
	}
	diffUpdated = len(res.elements) > 0 || len(res.removed) > 0
	err = s.headStorage.UpdateEntry(ctx, headstorage.HeadsUpdate{
		Id:    s.storageName,
		Heads: []string{s.diff.Hash()},
	})
	return
}

// updateResult carries one updateValues batch outcome: the new diff elements,
// what is needed to undo the diff on a failed tx (replaced elements' prior
// state, inserted ids, elements of watermark-dropped rows), and the watermarks
// to publish into s.watermarks once the tx commits.
type updateResult struct {
	elements   []ldiff.Element
	prior      []ldiff.Element
	added      []string
	removed    []ldiff.Element
	watermarks map[string]int64
}

// covered reports whether a row loses to an applied or in-batch watermark:
// its key falls under the prefix and it is strictly older. A watermark row
// itself never loses to its own prefix (equal timestamps are not covered).
func (s *storage) covered(res *updateResult, key string, timestampMicro int64) bool {
	for prefix, t := range s.watermarks {
		if timestampMicro < t && strings.HasPrefix(key, prefix) {
			return true
		}
	}
	for prefix, t := range res.watermarks {
		if timestampMicro < t && strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// updateValues upserts the values that win their LWW comparison. A winning
// watermark value additionally drops every stored row under its prefix that
// is older than it — document and diff element — leaving the watermark row
// as the only retained state.
func (s *storage) updateValues(ctx context.Context, values ...KeyValue) (res updateResult, err error) {
	parser := parserPool.Get()
	defer parserPool.Put(parser)
	arena := arenaPool.Get()
	defer arenaPool.Put(arena)

	res.elements = make([]ldiff.Element, 0, len(values))
	res.watermarks = map[string]int64{}
	var doc anystore.Doc
	for _, value := range values {
		if s.covered(&res, value.Key, value.TimestampMicro) {
			continue
		}
		doc, err = s.collection.FindIdWithParser(ctx, parser, value.KeyPeerId)
		isNotFound := errors.Is(err, anystore.ErrDocNotFound)
		if err != nil && !isNotFound {
			return
		}
		if !isNotFound {
			if int64(doc.Value().GetFloat64("t")) >= value.TimestampMicro {
				continue
			}
			res.prior = append(res.prior, anyEncToElement(doc.Value()))
		} else {
			res.added = append(res.added, value.KeyPeerId)
		}
		arena.Reset()
		val := value.AnyEnc(arena)
		if err = s.collection.UpsertOne(ctx, val); err != nil {
			return
		}
		res.elements = append(res.elements, anyEncToElement(val))
		if value.DeletePrefix != "" {
			if err = s.applyWatermark(ctx, &res, value); err != nil {
				return
			}
		}
	}
	return
}

// applyWatermark physically deletes every stored row whose key starts with
// the watermark's prefix and is strictly older than it, the just-written
// watermark row excepted by the timestamp comparison. Runs inside Set's tx;
// diff elements of the dropped rows are removed by Set after it returns.
func (s *storage) applyWatermark(ctx context.Context, res *updateResult, wm KeyValue) (err error) {
	// KeyPeerId starts with Key, so a key-prefix scan is an id-prefix scan.
	filter := query.Key{Path: []string{"id"}, Filter: query.NewComp(query.CompOpGte, wm.DeletePrefix)}
	iter, err := s.collection.Find(filter).Sort("id").Iter(ctx)
	if err != nil {
		return err
	}
	var dropIds []string
	var doc anystore.Doc
	for iter.Next() {
		if doc, err = iter.Doc(); err != nil {
			_ = iter.Close()
			return err
		}
		if !strings.HasPrefix(doc.Value().GetString("id"), wm.DeletePrefix) {
			break
		}
		if int64(doc.Value().GetFloat64("t")) >= wm.TimestampMicro {
			continue
		}
		res.removed = append(res.removed, anyEncToElement(doc.Value()))
		dropIds = append(dropIds, doc.Value().GetString("id"))
	}
	if err = iter.Close(); err != nil {
		return err
	}
	for _, id := range dropIds {
		if err = s.collection.DeleteId(ctx, id); err != nil {
			return err
		}
	}
	if wm.TimestampMicro > res.watermarks[wm.DeletePrefix] {
		res.watermarks[wm.DeletePrefix] = wm.TimestampMicro
	}
	return nil
}

func anyEncToElement(val *anyenc.Value) ldiff.Element {
	byteRepr := make([]byte, 8)
	binary.BigEndian.PutUint64(byteRepr, uint64(int64(val.GetFloat64("t"))))
	return ldiff.Element{
		Id:   val.GetString("id"),
		Head: string(byteRepr),
	}
}

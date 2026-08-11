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
	Set(ctx context.Context, keyValues ...KeyValue) (result SetResult, err error)
	Diff() ldiff.CompareDiff
	GetKeyPeerId(ctx context.Context, keyPeerId string) (keyValue KeyValue, err error)
	IterateValues(context.Context, func(kv KeyValue) (bool, error)) (err error)
	IteratePrefix(context.Context, string, func(kv KeyValue) error) (err error)
	// WatermarkTs returns the highest applied watermark timestamp covering
	// key, 0 when none. Serialized with Set by the caller (the outer storage
	// holds its mutex around every write).
	WatermarkTs(key string) int64
}

// SetResult reports what one Set call actually changed: the values that won
// their LWW comparison and were upserted, and the ids of rows a watermark in
// the batch physically removed. Values silently skipped — older than the
// stored row, or covered by a watermark — appear in neither.
type SetResult struct {
	Applied    []KeyValue
	DroppedIds []string
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
	return s.iterateIdPrefix(ctx, prefix, func(doc anystore.Doc) error {
		return iterFunc(s.keyValueFromDoc(doc))
	})
}

// iterateIdPrefix walks documents whose id starts with prefix, in id order:
// ids are sorted, so the matching rows form one contiguous run from the Gte
// seek, and the walk stops at the first id past it.
func (s *storage) iterateIdPrefix(ctx context.Context, prefix string, fn func(doc anystore.Doc) error) (err error) {
	filter := query.Key{Path: []string{"id"}, Filter: query.NewComp(query.CompOpGte, prefix)}
	iter, err := s.collection.Find(filter).Sort("id").Iter(ctx)
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
		if !strings.HasPrefix(doc.Value().GetString("id"), prefix) {
			break
		}
		if err = fn(doc); err != nil {
			return
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

func (s *storage) Set(ctx context.Context, values ...KeyValue) (result SetResult, err error) {
	tx, err := s.collection.WriteTx(ctx)
	if err != nil {
		return
	}
	var res updateResult
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
			return
		}
		// The diff already contains this call's mutations but the tx did not
		// commit: restore every touched element to its pre-call state so the
		// diff never advertises heads the storage doesn't hold — peers would
		// never re-send those values. preState records exactly one snapshot
		// per id, taken before its first mutation, so any interleaving of
		// upserts and watermark drops within the batch undoes correctly.
		for id, pre := range res.preState {
			if pre == nil {
				_ = s.diff.RemoveId(id)
			} else {
				s.diff.Set(*pre)
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
	if len(res.preState) == 0 {
		return SetResult{}, nil // nothing won LWW; skip the head update
	}
	err = s.headStorage.UpdateEntry(ctx, headstorage.HeadsUpdate{
		Id:    s.storageName,
		Heads: []string{s.diff.Hash()},
	})
	if err != nil {
		return
	}
	return SetResult{Applied: res.applied, DroppedIds: res.droppedIds}, nil
}

func (s *storage) WatermarkTs(key string) int64 {
	var maxTs int64
	for prefix, t := range s.watermarks {
		if t > maxTs && strings.HasPrefix(key, prefix) {
			maxTs = t
		}
	}
	return maxTs
}

// updateResult carries one updateValues batch outcome: the diff elements to
// advertise and withdraw, the per-id pre-call snapshots that undo the diff on
// a failed tx, the watermarks to publish into s.watermarks once the tx
// commits, and the applied/dropped sets reported to the caller.
type updateResult struct {
	elements []ldiff.Element
	removed  []ldiff.Element
	// preState maps every id mutated in this batch to its diff element
	// before the call (nil = absent), recorded once at first mutation.
	preState   map[string]*ldiff.Element
	watermarks map[string]int64
	applied    []KeyValue
	droppedIds []string
}

// recordPre snapshots an id's pre-call element once; later mutations of the
// same id within the batch keep the first (genuine) snapshot.
func (r *updateResult) recordPre(id string, el *ldiff.Element) {
	if _, ok := r.preState[id]; !ok {
		r.preState[id] = el
	}
}

// covered reports whether a row loses to an applied or in-batch watermark:
// its key falls under the prefix and it is strictly older. A watermark row
// itself never loses to its own prefix (equal timestamps are not covered).
func (s *storage) covered(res *updateResult, key string, timestampMicro int64) bool {
	match := func(watermarks map[string]int64) bool {
		for prefix, t := range watermarks {
			if timestampMicro < t && strings.HasPrefix(key, prefix) {
				return true
			}
		}
		return false
	}
	return match(s.watermarks) || match(res.watermarks)
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
	res.preState = map[string]*ldiff.Element{}
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
			el := anyEncToElement(doc.Value())
			res.recordPre(el.Id, &el)
		} else {
			res.recordPre(value.KeyPeerId, nil)
		}
		arena.Reset()
		val := value.AnyEnc(arena)
		if err = s.collection.UpsertOne(ctx, val); err != nil {
			return
		}
		res.elements = append(res.elements, anyEncToElement(val))
		res.applied = append(res.applied, value)
		if value.DeletePrefix != "" {
			if err = s.applyWatermark(ctx, &res, value); err != nil {
				return
			}
		}
	}
	return
}

// applyWatermark physically deletes every stored row whose KEY starts with
// the watermark's prefix and is strictly older than it, the just-written
// watermark row excepted by the timestamp comparison. Runs inside Set's tx;
// diff elements of the dropped rows are removed by Set after it returns.
func (s *storage) applyWatermark(ctx context.Context, res *updateResult, wm KeyValue) (err error) {
	var dropIds []string
	// A key under the prefix always puts its ids (key+"-"+peerId) inside the
	// id-prefix run, so the scan is bounded — but not vice versa: an id can
	// match past its key's end ("cnt-…" peer ids under prefix "cnt-1" belong
	// to key "cnt"), so the key itself must be checked per document.
	err = s.iterateIdPrefix(ctx, wm.DeletePrefix, func(doc anystore.Doc) error {
		if !strings.HasPrefix(doc.Value().GetString("k"), wm.DeletePrefix) {
			return nil
		}
		if int64(doc.Value().GetFloat64("t")) >= wm.TimestampMicro {
			return nil
		}
		el := anyEncToElement(doc.Value())
		res.recordPre(el.Id, &el)
		res.removed = append(res.removed, el)
		dropIds = append(dropIds, el.Id)
		return nil
	})
	if err != nil {
		return err
	}
	for _, id := range dropIds {
		if err = s.collection.DeleteId(ctx, id); err != nil {
			return err
		}
	}
	res.droppedIds = append(res.droppedIds, dropIds...)
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

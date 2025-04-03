package innerstorage

import (
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
	Get(ctx context.Context, key string) (keyValue KeyValue, err error)
	IterateValues(context.Context, func(kv KeyValue) (bool, error)) (err error)
	IteratePrefix(context.Context, string, func(kv KeyValue) error) (err error)
}

type storage struct {
	diff        ldiff.CompareDiff
	headStorage headstorage.HeadStorage
	collection  anystore.Collection
	store       anystore.DB
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
	err = headStorage.UpdateEntryTx(tx.Context(), headstorage.HeadsUpdate{
		Id:    storageName,
		Heads: []string{hash},
	})
	return storage, err
}

func (s *storage) Diff() ldiff.CompareDiff {
	return s.diff
}

func (s *storage) Get(ctx context.Context, key string) (value KeyValue, err error) {
	doc, err := s.collection.FindId(ctx, key)
	if err != nil {
		if errors.Is(err, anystore.ErrDocNotFound) {
			return KeyValue{}, nil
		}
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
	iter, err := s.collection.Find(qry).Iter(ctx)
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

func (s *storage) keyValueFromDoc(doc anystore.Doc) KeyValue {
	valueObj := doc.Value().GetObject("v")
	value := Value{
		Value:             valueObj.Get("v").GetBytes(),
		PeerSignature:     valueObj.Get("p").GetBytes(),
		IdentitySignature: valueObj.Get("i").GetBytes(),
	}
	return KeyValue{
		Key:            doc.Value().GetString("id"),
		Value:          value,
		TimestampMilli: doc.Value().GetInt("t"),
		Identity:       doc.Value().GetString("i"),
		PeerId:         doc.Value().GetString("p"),
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
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()
	ctx = tx.Context()
	elements, err := s.updateValues(ctx, values...)
	if err != nil {
		return
	}
	s.diff.Set(elements...)
	return
}

func (s *storage) updateValues(ctx context.Context, values ...KeyValue) (elements []ldiff.Element, err error) {
	parser := parserPool.Get()
	defer parserPool.Put(parser)
	arena := arenaPool.Get()
	defer arenaPool.Put(arena)

	elements = make([]ldiff.Element, 0, len(values))
	var doc anystore.Doc
	for _, value := range values {
		doc, err = s.collection.FindIdWithParser(ctx, parser, value.Key)
		isNotFound := errors.Is(err, anystore.ErrDocNotFound)
		if err != nil && !isNotFound {
			return
		}
		if !isNotFound {
			if doc.Value().GetInt("t") >= value.TimestampMilli {
				continue
			}
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
	binary.BigEndian.PutUint64(byteRepr, uint64(val.GetInt("t")))
	return ldiff.Element{
		Id:   val.GetString("id"),
		Head: string(byteRepr),
	}
}

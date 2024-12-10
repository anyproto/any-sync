package keyvaluestore

import (
	"context"
	"errors"
	"strconv"

	anystore "github.com/anyproto/any-store"
	"github.com/anyproto/any-store/anyenc"

	"github.com/anyproto/any-sync/app/ldiff"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue"
)

var (
	parserPool = &anyenc.ParserPool{}
	arenaPool  = &anyenc.ArenaPool{}
)

/**

any-store document structure:
	id (string) - key
	v (bytes) - value
	h (string) - value hash
	t (int) - timestamp
	d (bool) - isDeleted
*/

func NewKeyValueStore(ctx context.Context, collection anystore.Collection) (KeyValueStore, error) {
	k := &keyValueStore{collection: collection}
	if err := k.init(ctx); err != nil {
		return nil, err
	}
	return k, nil
}

type KeyValueStore interface {
	Set(ctx context.Context, values ...keyvalue.KeyValue) (err error)
	ldiff.Remote
}

type keyValueStore struct {
	collection anystore.Collection
	ldiff.Diff
}

func (k *keyValueStore) init(ctx context.Context) (err error) {
	k.Diff = ldiff.New(32, 256)
	iter, err := k.collection.Find(nil).Iter(ctx)
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
		k.Diff.Set(anyEncToElement(doc.Value()))
	}
	return
}

func (k *keyValueStore) Set(ctx context.Context, values ...keyvalue.KeyValue) (err error) {
	elements, err := k.updateValues(ctx, values...)
	if err != nil {
		return
	}
	k.Diff.Set(elements...)
	return
}

func (k *keyValueStore) updateValues(ctx context.Context, values ...keyvalue.KeyValue) (elements []ldiff.Element, err error) {
	tx, err := k.collection.WriteTx(ctx)
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
	parser := parserPool.Get()
	defer parserPool.Put(parser)
	arena := arenaPool.Get()
	defer arenaPool.Put(arena)

	elements = make([]ldiff.Element, 0, len(values))
	var doc anystore.Doc
	for _, value := range values {
		doc, err = k.collection.FindIdWithParser(ctx, parser, value.Key)
		isNotFound := errors.Is(err, anystore.ErrDocNotFound)
		if err != nil && !isNotFound {
			return
		}
		if !isNotFound {
			if doc.Value().GetInt("t") >= value.TimestampMilli {
				// newest doc is already present
				continue
			}
		}
		arena.Reset()
		val := value.AnyEnc(arena)
		if err = k.collection.UpsertOne(ctx, val); err != nil {
			return
		}
		elements = append(elements, anyEncToElement(val))
	}
	return
}

func anyEncToElement(val *anyenc.Value) ldiff.Element {
	head := strconv.Itoa(val.GetInt("t")) + "/"
	if val.GetBool("d") {
		head += "del"
	} else {
		head += val.GetString("h")
	}
	return ldiff.Element{
		Id:   val.GetString("id"),
		Head: head,
	}
}

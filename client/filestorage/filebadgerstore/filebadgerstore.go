package filebadgerstore

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileblockstore"
	"github.com/dgraph-io/badger/v3"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
)

const keyPrefix = "files/blocks/"

func NewBadgerStorage(db *badger.DB) fileblockstore.BlockStoreExistsCIDs {
	return &badgerStorage{db: db}
}

type badgerStorage struct {
	db *badger.DB
}

func (f *badgerStorage) Get(ctx context.Context, k cid.Cid) (b blocks.Block, err error) {
	err = f.db.View(func(txn *badger.Txn) error {
		it, gerr := txn.Get(key(k))
		if gerr != nil {
			return gerr
		}
		return it.Value(func(val []byte) error {
			b = blocks.NewBlock(val)
			return nil
		})
	})
	return
}

func (f *badgerStorage) GetMany(ctx context.Context, ks []cid.Cid) <-chan blocks.Block {
	var res = make(chan blocks.Block)
	go func() {
		defer close(res)
		_ = f.db.View(func(txn *badger.Txn) error {
			// TODO: log errors
			for _, k := range ks {
				it, gerr := txn.Get(key(k))
				if gerr != nil {
					return gerr
				}
				_ = it.Value(func(val []byte) error {
					res <- blocks.NewBlock(val)
					return nil
				})
			}
			return nil
		})
	}()
	return res
}

func (f *badgerStorage) Add(ctx context.Context, bs []blocks.Block) error {
	return f.db.Update(func(txn *badger.Txn) error {
		for _, b := range bs {
			if err := txn.Set(key(b.Cid()), b.RawData()); err != nil {
				return err
			}
		}
		return nil
	})
}

func (f *badgerStorage) Delete(ctx context.Context, c cid.Cid) error {
	return f.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key(c))
	})
}

func (f *badgerStorage) ExistsCids(ctx context.Context, ks []cid.Cid) (exists []cid.Cid, err error) {
	err = f.db.View(func(txn *badger.Txn) error {
		for _, k := range ks {
			if _, e := txn.Get(key(k)); e == nil {
				exists = append(exists, k)
			}
		}
		return nil
	})
	return
}

func key(c cid.Cid) []byte {
	return []byte(keyPrefix + c.String())
}

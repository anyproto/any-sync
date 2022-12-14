package cachestore

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileblockstore"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"go.uber.org/zap"
	"io"
)

type CacheStore struct {
	Cache  fileblockstore.BlockStoreExistsCIDs
	Origin fileblockstore.BlockStore
	Log    *zap.Logger
}

func (c *CacheStore) Get(ctx context.Context, k cid.Cid) (b blocks.Block, err error) {
	if b, err = c.Cache.Get(ctx, k); err != nil {
		if format.IsNotFound(err) {
			err = nil
		} else {
			return
		}
	} else {
		return
	}
	if b, err = c.Origin.Get(ctx, k); err != nil {
		return
	}
	if addErr := c.Cache.Add(ctx, []blocks.Block{b}); addErr != nil {
		c.Log.Error("block fetched from origin but got error for add to cache", zap.Error(addErr))
	}
	return
}

func (c *CacheStore) GetMany(ctx context.Context, ks []cid.Cid) <-chan blocks.Block {
	cachedCids, localErr := c.Cache.ExistsCids(ctx, ks)
	var originCids []cid.Cid
	if localErr != nil {
		c.Log.Error("hasCIDs error", zap.Error(localErr))
		originCids = ks
	} else {
		if len(cachedCids) != len(ks) {
			set := cid.NewSet()
			for _, cCid := range cachedCids {
				set.Add(cCid)
			}
			originCids = ks[:0]
			for _, k := range ks {
				if !set.Has(k) {
					originCids = append(originCids, k)
				}
			}
		}
	}

	if len(originCids) == 0 {
		return c.Cache.GetMany(ctx, cachedCids)
	}

	var results = make(chan blocks.Block)

	go func() {
		defer close(results)
		localResults := c.Cache.GetMany(ctx, cachedCids)
		originResults := c.Origin.GetMany(ctx, originCids)
		for {
			var cb, ob blocks.Block
			var cOk, oOk bool
			select {
			case cb, cOk = <-localResults:
				if cOk {
					results <- cb
				}
			case ob, oOk = <-originResults:
				if oOk {
					if addErr := c.Cache.Add(ctx, []blocks.Block{ob}); addErr != nil {
						c.Log.Error("add block to cache error", zap.Error(addErr))
					}
					results <- ob
				}
			case <-ctx.Done():
				return
			}
			if !oOk && !cOk {
				return
			}
		}
	}()

	return results
}

func (c *CacheStore) Add(ctx context.Context, b []blocks.Block) error {
	if localErr := c.Cache.Add(ctx, b); localErr != nil {
		c.Log.Error("cache add error", zap.Error(localErr))
	}
	return c.Origin.Add(ctx, b)
}

func (c *CacheStore) Delete(ctx context.Context, k cid.Cid) error {
	if localErr := c.Cache.Delete(ctx, k); localErr != nil {
		if !format.IsNotFound(localErr) {
			c.Log.Error("error while delete block", zap.Error(localErr))
		}
	}
	return c.Origin.Delete(ctx, k)
}

func (c *CacheStore) Close() (err error) {
	if closer, ok := c.Cache.(io.Closer); ok {
		if localErr := closer.Close(); localErr != nil {
			c.Log.Error("error while closing cache store", zap.Error(localErr))
		}
	}
	if closer, ok := c.Origin.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

package filestorage

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/filestorage/badgerfilestore"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileblockstore"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"go.uber.org/zap"
	"io"
)

type proxyStore struct {
	cache  fileblockstore.BlockStoreLocal
	origin fileblockstore.BlockStore
	index  *badgerfilestore.FileBadgerIndex
}

func (c *proxyStore) Get(ctx context.Context, k cid.Cid) (b blocks.Block, err error) {
	log.Debug("get cid", zap.String("cid", k.String()))
	if b, err = c.cache.Get(ctx, k); err != nil {
		if format.IsNotFound(err) {
			err = nil
			log.Debug("proxyStore local cid not found", zap.String("cid", k.String()))
		} else {
			return
		}
	} else {
		return
	}
	if b, err = c.origin.Get(ctx, k); err != nil {
		log.Debug("proxyStore remote cid error", zap.String("cid", k.String()), zap.Error(err))
		return
	}
	if addErr := c.cache.Add(ctx, []blocks.Block{b}); addErr != nil {
		log.Error("block fetched from origin but got error for add to cache", zap.Error(addErr))
	}
	return
}

func (c *proxyStore) GetMany(ctx context.Context, ks []cid.Cid) <-chan blocks.Block {
	cachedCids, localErr := c.cache.ExistsCids(ctx, ks)
	var originCids []cid.Cid
	if localErr != nil {
		log.Error("proxy store hasCIDs error", zap.Error(localErr))
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
	log.Debug("get many cids", zap.Int("cached", len(cachedCids)), zap.Int("origin", len(originCids)))
	if len(originCids) == 0 {
		return c.cache.GetMany(ctx, cachedCids)
	}
	var results = make(chan blocks.Block)

	go func() {
		defer close(results)
		localResults := c.cache.GetMany(ctx, cachedCids)
		originResults := c.origin.GetMany(ctx, originCids)
		oOk, cOk := true, true
		for {
			var cb, ob blocks.Block
			select {
			case cb, cOk = <-localResults:
				if cOk {
					results <- cb
				}
			case ob, oOk = <-originResults:
				if oOk {
					if addErr := c.cache.Add(ctx, []blocks.Block{ob}); addErr != nil {
						log.Error("add block to cache error", zap.Error(addErr))
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

func (c *proxyStore) Add(ctx context.Context, bs []blocks.Block) error {
	indexCids := badgerfilestore.NewCids()
	defer indexCids.Release()
	for _, b := range bs {
		indexCids.Add(fileblockstore.CtxGetSpaceId(ctx), badgerfilestore.OpAdd, b.Cid())
	}
	if err := c.index.Add(indexCids); err != nil {
		return err
	}
	return c.cache.Add(ctx, bs)
}

func (c *proxyStore) Delete(ctx context.Context, k cid.Cid) error {
	indexCids := badgerfilestore.NewCids()
	defer indexCids.Release()
	indexCids.Add(fileblockstore.CtxGetSpaceId(ctx), badgerfilestore.OpDelete, k)
	if err := c.index.Add(indexCids); err != nil {
		return err
	}
	return c.cache.Delete(ctx, k)
}

func (c *proxyStore) Close() (err error) {
	if closer, ok := c.cache.(io.Closer); ok {
		if localErr := closer.Close(); localErr != nil {
			log.Error("error while closing cache store", zap.Error(localErr))
		}
	}
	if closer, ok := c.origin.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

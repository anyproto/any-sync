package cachestore

import (
	"context"
	"fmt"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

var ctx = context.Background()

func TestCacheStore_Add(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cs := &CacheStore{
			Cache:  newTestStore(nil),
			Origin: newTestStore(nil),
		}
		defer cs.Close()
		testBlocks := newTestBocks("1", "2", "3")
		require.NoError(t, cs.Add(ctx, testBlocks))
		for _, b := range testBlocks {
			gb, err := cs.Cache.Get(ctx, b.Cid())
			assert.NoError(t, err)
			assert.NotNil(t, gb)
		}
	})
}

func TestCacheStore_Get(t *testing.T) {
	t.Run("exists local", func(t *testing.T) {
		testBlocks := newTestBocks("1", "2", "3")
		cs := &CacheStore{
			Cache:  newTestStore(testBlocks),
			Origin: newTestStore(testBlocks),
		}
		defer cs.Close()
		for _, b := range testBlocks {
			gb, err := cs.Get(ctx, b.Cid())
			assert.NoError(t, err)
			assert.NotNil(t, gb)
		}
	})
	t.Run("exists remote", func(t *testing.T) {
		testBlocks := newTestBocks("1", "2", "3")
		cs := &CacheStore{
			Cache:  newTestStore(testBlocks[:1]),
			Origin: newTestStore(testBlocks),
		}
		defer cs.Close()
		for _, b := range testBlocks {
			gb, err := cs.Get(ctx, b.Cid())
			assert.NoError(t, err)
			assert.NotNil(t, gb)
		}
		for _, b := range testBlocks {
			lb, err := cs.Cache.Get(ctx, b.Cid())
			assert.NoError(t, err)
			assert.NotNil(t, lb)
		}
	})
}

func TestCacheStore_GetMany(t *testing.T) {
	t.Run("all local", func(t *testing.T) {
		testBlocks := newTestBocks("1", "2", "3")
		cs := &CacheStore{
			Cache:  newTestStore(testBlocks),
			Origin: newTestStore(testBlocks),
		}
		defer cs.Close()

		var cids, resCids []cid.Cid
		for _, b := range testBlocks {
			cids = append(cids, b.Cid())
		}
		ch := cs.GetMany(ctx, cids)
		for {
			select {
			case b, ok := <-ch:
				if !ok {
					return
				} else {
					resCids = append(resCids, b.Cid())
				}
			case <-time.After(time.Second):
				assert.NoError(t, fmt.Errorf("timeout"))
				return
			}
		}
		assert.ElementsMatch(t, cids, resCids)
	})
	t.Run("partial local", func(t *testing.T) {
		testBlocks := newTestBocks("1", "2", "3")
		cs := &CacheStore{
			Cache:  newTestStore(testBlocks[:1]),
			Origin: newTestStore(testBlocks),
		}
		defer cs.Close()

		var cids, resCids []cid.Cid
		for _, b := range testBlocks {
			cids = append(cids, b.Cid())
		}
		ch := cs.GetMany(ctx, cids)
		for {
			select {
			case b, ok := <-ch:
				if !ok {
					return
				} else {
					resCids = append(resCids, b.Cid())
				}
			case <-time.After(time.Second):
				assert.NoError(t, fmt.Errorf("timeout"))
				return
			}
		}
		assert.ElementsMatch(t, cids, resCids)
		for _, b := range testBlocks {
			gb, err := cs.Cache.Get(ctx, b.Cid())
			assert.NoError(t, err)
			assert.NotNil(t, gb)
		}
	})
}

func TestCacheStore_Delete(t *testing.T) {
	testBlocks := newTestBocks("1", "2", "3")
	cs := &CacheStore{
		Cache:  newTestStore(testBlocks[:1]),
		Origin: newTestStore(testBlocks),
	}
	defer cs.Close()
	for _, b := range testBlocks {
		require.NoError(t, cs.Delete(ctx, b.Cid()))
		gb, err := cs.Get(ctx, b.Cid())
		assert.Nil(t, gb)
		assert.True(t, format.IsNotFound(err))
	}
}

func newTestStore(bs []blocks.Block) *testStore {
	ts := &testStore{
		store: make(map[cid.Cid]blocks.Block),
	}
	ts.Add(context.Background(), bs)
	return ts
}

type testStore struct {
	store map[cid.Cid]blocks.Block
}

func (t *testStore) Get(ctx context.Context, k cid.Cid) (blocks.Block, error) {
	if b, ok := t.store[k]; ok {
		return b, nil
	}
	return nil, &format.ErrNotFound{Cid: k}
}

func (t *testStore) GetMany(ctx context.Context, ks []cid.Cid) <-chan blocks.Block {
	var result = make(chan blocks.Block)
	go func() {
		defer close(result)
		for _, k := range ks {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if b, err := t.Get(ctx, k); err == nil {
				result <- b
			}
		}
	}()
	return result
}

func (t *testStore) ExistsCids(ctx context.Context, ks []cid.Cid) (exists []cid.Cid, err error) {
	for _, k := range ks {
		if _, ok := t.store[k]; ok {
			exists = append(exists, k)
		}
	}
	return
}

func (t *testStore) Add(ctx context.Context, bs []blocks.Block) error {
	for _, b := range bs {
		t.store[b.Cid()] = b
	}
	return nil
}

func (t *testStore) Delete(ctx context.Context, c cid.Cid) error {
	if _, ok := t.store[c]; ok {
		delete(t.store, c)
		return nil
	}
	return &format.ErrNotFound{Cid: c}
}

func (t *testStore) Close() (err error) {
	return nil
}

func newTestBocks(ids ...string) (bs []blocks.Block) {
	for _, id := range ids {
		bs = append(bs, blocks.NewBlock([]byte(id)))
	}
	return
}

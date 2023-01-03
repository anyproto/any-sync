package filestorage

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/filestorage/badgerfilestore"
	"github.com/dgraph-io/badger/v3"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"sync"
	"testing"
	"time"
)

var ctx = context.Background()

func TestCacheStore_Add(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cs := newPSFixture(t)
		defer cs.Finish(t)
		testBlocks := newTestBocks("1", "2", "3")
		require.NoError(t, cs.Add(ctx, testBlocks))
		for _, b := range testBlocks {
			gb, err := cs.cache.Get(ctx, b.Cid())
			assert.NoError(t, err)
			assert.NotNil(t, gb)
		}
		cids, err := cs.index.List(100)
		require.NoError(t, err)
		require.Len(t, cids.SpaceOps, 1)
		assert.Len(t, cids.SpaceOps[0].Add, len(testBlocks))
	})
}

func TestCacheStore_Get(t *testing.T) {
	t.Run("exists local", func(t *testing.T) {
		testBlocks := newTestBocks("1", "2", "3")
		cs := newPSFixture(t)
		defer cs.Finish(t)
		require.NoError(t, cs.cache.Add(ctx, testBlocks))
		require.NoError(t, cs.origin.Add(ctx, testBlocks))
		for _, b := range testBlocks {
			gb, err := cs.Get(ctx, b.Cid())
			assert.NoError(t, err)
			assert.NotNil(t, gb)
		}
	})
	t.Run("exists remote", func(t *testing.T) {
		testBlocks := newTestBocks("1", "2", "3")
		cs := newPSFixture(t)
		defer cs.Finish(t)
		require.NoError(t, cs.cache.Add(ctx, testBlocks[:1]))
		require.NoError(t, cs.origin.Add(ctx, testBlocks))
		for _, b := range testBlocks {
			gb, err := cs.Get(ctx, b.Cid())
			assert.NoError(t, err)
			assert.NotNil(t, gb)
		}
		for _, b := range testBlocks {
			lb, err := cs.cache.Get(ctx, b.Cid())
			assert.NoError(t, err)
			assert.NotNil(t, lb)
		}
	})
}

func TestCacheStore_GetMany(t *testing.T) {
	t.Run("all local", func(t *testing.T) {
		testBlocks := newTestBocks("1", "2", "3")
		cs := newPSFixture(t)
		defer cs.Finish(t)
		require.NoError(t, cs.cache.Add(ctx, testBlocks))
		require.NoError(t, cs.origin.Add(ctx, testBlocks))

		var cids, resCids []cid.Cid
		for _, b := range testBlocks {
			cids = append(cids, b.Cid())
		}
		ch := cs.GetMany(ctx, cids)
		func() {
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
		}()
		assert.ElementsMatch(t, cids, resCids)
	})
	t.Run("partial local", func(t *testing.T) {
		testBlocks := newTestBocks("1", "2", "3")
		cs := newPSFixture(t)
		defer cs.Finish(t)
		require.NoError(t, cs.cache.Add(ctx, testBlocks[:1]))
		require.NoError(t, cs.origin.Add(ctx, testBlocks))

		var cids, resCids []cid.Cid
		for _, b := range testBlocks {
			cids = append(cids, b.Cid())
		}
		ch := cs.GetMany(ctx, cids)
		func() {
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
		}()
		require.Equal(t, len(cids), len(resCids))
		for _, b := range testBlocks {
			gb, err := cs.cache.Get(ctx, b.Cid())
			assert.NoError(t, err)
			assert.NotNil(t, gb)
		}
	})
}

func TestCacheStore_Delete(t *testing.T) {
	testBlocks := newTestBocks("1", "2", "3")
	cs := newPSFixture(t)
	defer cs.Finish(t)
	require.NoError(t, cs.cache.Add(ctx, testBlocks))
	for _, b := range testBlocks {
		require.NoError(t, cs.Delete(ctx, b.Cid()))
		gb, err := cs.cache.Get(ctx, b.Cid())
		assert.Nil(t, gb)
		assert.True(t, format.IsNotFound(err))
	}
}

func newTestStore(bs []blocks.Block) *testStore {
	ts := &testStore{
		store: make(map[string]blocks.Block),
	}
	ts.Add(context.Background(), bs)
	return ts
}

type testStore struct {
	store map[string]blocks.Block
	mu    sync.Mutex
}

func (t *testStore) NotExistsBlocks(ctx context.Context, bs []blocks.Block) (notExists []blocks.Block, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	notExists = bs[:0]
	for _, b := range bs {
		if _, ok := t.store[b.Cid().String()]; !ok {
			notExists = append(notExists, b)
		}
	}
	return
}

func (t *testStore) Get(ctx context.Context, k cid.Cid) (blocks.Block, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if b, ok := t.store[k.String()]; ok {
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
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, k := range ks {
		if _, ok := t.store[k.String()]; ok {
			exists = append(exists, k)
		}
	}
	return
}

func (t *testStore) Add(ctx context.Context, bs []blocks.Block) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, b := range bs {
		t.store[b.Cid().String()] = b
	}
	return nil
}

func (t *testStore) AddAsync(ctx context.Context, bs []blocks.Block) (successCh chan cid.Cid) {
	successCh = make(chan cid.Cid, len(bs))
	go func() {
		defer close(successCh)
		for _, b := range bs {
			if err := t.Add(ctx, []blocks.Block{b}); err == nil {
				successCh <- b.Cid()
			}
		}
	}()
	return successCh
}

func (t *testStore) Delete(ctx context.Context, c cid.Cid) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.store[c.String()]; ok {
		delete(t.store, c.String())
		return nil
	}
	return &format.ErrNotFound{Cid: c}
}

func (t *testStore) Close() (err error) {
	return nil
}

type psFixture struct {
	*proxyStore
	tmpDir string
	db     *badger.DB
}

func newPSFixture(t *testing.T) *psFixture {
	var err error
	fx := &psFixture{}
	fx.tmpDir, err = os.MkdirTemp("", "proxyStore_*")
	require.NoError(t, err)
	fx.db, err = badger.Open(badger.DefaultOptions(fx.tmpDir).WithLoggingLevel(badger.ERROR))
	require.NoError(t, err)
	fx.proxyStore = &proxyStore{
		cache:  newTestStore(nil),
		origin: newTestStore(nil),
		index:  badgerfilestore.NewFileBadgerIndex(fx.db),
	}
	return fx
}

func (fx *psFixture) Finish(t *testing.T) {
	assert.NoError(t, fx.db.Close())
	_ = os.RemoveAll(fx.tmpDir)
}

func newTestBocks(ids ...string) (bs []blocks.Block) {
	for _, id := range ids {
		bs = append(bs, blocks.NewBlock([]byte(id)))
	}
	return
}

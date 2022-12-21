package badgerfilestore

import (
	"bytes"
	"fmt"
	"github.com/dgraph-io/badger/v3"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestFileBadgerIndex_Add(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badgerfileindextest_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	db, err := badger.Open(badger.DefaultOptions(tmpDir))
	require.NoError(t, err)
	defer db.Close()
	index := NewFileBadgerIndex(db)
	var cids = NewCids()
	defer cids.Release()
	for _, spaceId := range []string{"space1", "space2"} {
		for i := 0; i < 5; i++ {
			cids.Add(spaceId, OpAdd, blocks.NewBlock([]byte(fmt.Sprint("add", i))).Cid())
		}
		for i := 0; i < 3; i++ {
			cids.Add(spaceId, OpDelete, blocks.NewBlock([]byte(fmt.Sprint("del", i))).Cid())
		}
		for i := 0; i < 2; i++ {
			cids.Add(spaceId, OpLoad, blocks.NewBlock([]byte(fmt.Sprint("load", i))).Cid())
		}
	}
	require.NoError(t, index.Add(cids))

	cids, err = index.List(100)
	require.NoError(t, err)
	assert.Len(t, cids.SpaceOps, 2)
	for _, s := range cids.SpaceOps {
		assert.Len(t, s.Add, 5)
		assert.Len(t, s.Delete, 3)
		assert.Len(t, s.Load, 2)
	}
}

func TestCids_Add(t *testing.T) {
	for i := 0; i < 3; i++ {
		var bs = []blocks.Block{
			blocks.NewBlock([]byte("1")),
			blocks.NewBlock([]byte("2")),
			blocks.NewBlock([]byte("3")),
			blocks.NewBlock([]byte("4")),
			blocks.NewBlock([]byte("5")),
		}
		cids := NewCids()
		for _, b := range bs {
			cids.Add("1", OpAdd, b.Cid())
		}
		keys := cids.Keys()
		contains := func(c cid.Cid) bool {
			for _, k := range keys {
				if bytes.HasSuffix(k, c.Bytes()) {
					return true
				}
			}
			return false
		}
		for _, b := range bs {
			assert.True(t, contains(b.Cid()))
		}
		kcids := NewCids()
		for _, k := range keys {
			require.NoError(t, kcids.AddKey(k))
		}

		require.Len(t, kcids.SpaceOps, 1)
		assert.Equal(t, kcids.SpaceOps[0].Add, cids.SpaceOps[0].Add)

		cids.Release()
		kcids.Release()
	}
}

func BenchmarkCids_Add(b *testing.B) {
	var bs = []blocks.Block{
		blocks.NewBlock([]byte("1")),
		blocks.NewBlock([]byte("2")),
		blocks.NewBlock([]byte("3")),
		blocks.NewBlock([]byte("4")),
		blocks.NewBlock([]byte("5")),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cids := NewCids()
		for _, b := range bs {
			cids.Add("1", OpAdd, b.Cid())
		}
		cids.Keys()
		cids.Release()
	}
}

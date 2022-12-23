package badgerfilestore

import (
	"context"
	"github.com/dgraph-io/badger/v3"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

var ctx = context.Background()

func TestBadgerStorage_Add(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badgerfilestore_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	db, err := badger.Open(badger.DefaultOptions(tmpDir))
	require.NoError(t, err)
	defer db.Close()

	s := NewBadgerStorage(db)
	bs := []blocks.Block{
		blocks.NewBlock([]byte("1")),
		blocks.NewBlock([]byte("2")),
		blocks.NewBlock([]byte("3")),
	}
	assert.NoError(t, s.Add(ctx, bs))

}

func TestBadgerStorage_Get(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badgerfilestore_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	db, err := badger.Open(badger.DefaultOptions(tmpDir))
	require.NoError(t, err)
	defer db.Close()

	s := NewBadgerStorage(db)
	bs := []blocks.Block{
		blocks.NewBlock([]byte("1")),
		blocks.NewBlock([]byte("2")),
		blocks.NewBlock([]byte("3")),
	}
	require.NoError(t, s.Add(ctx, bs))

	cids := make([]cid.Cid, 0, len(bs))
	for _, b := range bs {
		cids = append(cids, b.Cid())
	}
	cids = append(cids, blocks.NewBlock([]byte("4")).Cid())

	b, err := s.Get(ctx, bs[0].Cid())
	require.NoError(t, err)
	assert.Equal(t, bs[0].RawData(), b.RawData())

	ecids, err := s.ExistsCids(ctx, cids)
	require.NoError(t, err)
	assert.Len(t, ecids, 3)
}

func TestBadgerStorage_GetMany(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badgerfilestore_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	db, err := badger.Open(badger.DefaultOptions(tmpDir))
	require.NoError(t, err)
	defer db.Close()

	s := NewBadgerStorage(db)
	bs := []blocks.Block{
		blocks.NewBlock([]byte("1")),
		blocks.NewBlock([]byte("2")),
		blocks.NewBlock([]byte("3")),
	}
	require.NoError(t, s.Add(ctx, bs))

	cids := make([]cid.Cid, 0, len(bs))
	for _, b := range bs {
		cids = append(cids, b.Cid())
	}

	res := s.GetMany(ctx, cids)
	var resB []blocks.Block
	for i := 0; i < len(bs); i++ {
		select {
		case b := <-res:
			resB = append(resB, b)
		case <-time.After(time.Second):
			t.Error("timeout")
			return
		}
	}
	assert.Len(t, resB, 3)
	_, ok := <-res
	assert.False(t, ok)
}

package keyvaluestore

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	anystore "github.com/anyproto/any-store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/keyvalue"
)

var ctx = context.Background()

func TestKeyValueStore_Set(t *testing.T) {
	var values = []keyvalue.KeyValue{
		{
			Key:            "1",
			Value:          []byte("1"),
			TimestampMilli: 1,
		},
		{
			Key:            "2",
			Value:          []byte("2"),
			TimestampMilli: 1,
		},
		{
			Key:            "3",
			Value:          []byte("4"),
			TimestampMilli: 1,
		},
	}

	t.Run("new", func(t *testing.T) {
		fx := newFixture(t)
		err := fx.Set(ctx, values...)
		assert.NoError(t, err)
		count, _ := fx.coll.Count(ctx)
		assert.Equal(t, 3, count)
		assert.Equal(t, 3, fx.Diff.Len())
	})
	t.Run("restore", func(t *testing.T) {
		fx := newFixture(t)
		err := fx.Set(ctx, values...)
		assert.NoError(t, err)
		hash := fx.Diff.Hash()
		kv2, err := NewKeyValueStore(ctx, fx.coll)
		require.NoError(t, err)
		assert.Equal(t, kv2.(*keyValueStore).Diff.Hash(), hash)
	})
	t.Run("update", func(t *testing.T) {
		fx := newFixture(t)
		err := fx.Set(ctx, values...)
		assert.NoError(t, err)
		hash := fx.Diff.Hash()

		// do not update because timestamp is not greater
		require.NoError(t, fx.Set(ctx, keyvalue.KeyValue{Key: "1", TimestampMilli: 1}))
		assert.Equal(t, hash, fx.Diff.Hash())
		// should be updated
		require.NoError(t, fx.Set(ctx, keyvalue.KeyValue{Key: "2", TimestampMilli: 2, Value: []byte("22")}))
		assert.NotEqual(t, hash, fx.Diff.Hash())
	})
	t.Run("delete", func(t *testing.T) {
		fx := newFixture(t)
		err := fx.Set(ctx, values...)
		assert.NoError(t, err)
		hash := fx.Diff.Hash()

		require.NoError(t, fx.Set(ctx, keyvalue.KeyValue{Key: "1", TimestampMilli: 3}))
		assert.NotEqual(t, hash, fx.Diff.Hash())
	})
}

func TestKeyValueStore_E2E(t *testing.T) {
	fx1 := newFixture(t)
	fx2 := newFixture(t)

	newIds, changedIds, remodedIds, err := fx1.Diff.Diff(ctx, fx2)
	require.NoError(t, err)
	assert.Len(t, newIds, 0)
	assert.Len(t, changedIds, 0)
	assert.Len(t, remodedIds, 0)

	require.NoError(t, fx1.Set(ctx, []keyvalue.KeyValue{
		{Key: "1", Value: []byte("1"), TimestampMilli: 1},
		{Key: "2", Value: []byte("1"), TimestampMilli: 1},
		{Key: "3", Value: []byte("1"), TimestampMilli: 1},
		{Key: "4", Value: []byte("1"), TimestampMilli: 1},
		{Key: "5", Value: []byte("1"), TimestampMilli: 1},
	}...))

	newIds, changedIds, remodedIds, err = fx1.Diff.Diff(ctx, fx2)
	require.NoError(t, err)
	assert.Len(t, newIds, 0)
	assert.Len(t, changedIds, 0)
	assert.Len(t, remodedIds, 5)

	require.NoError(t, fx2.Set(ctx, []keyvalue.KeyValue{
		{Key: "1", Value: []byte("1"), TimestampMilli: 1},
		{Key: "2", Value: []byte("1"), TimestampMilli: 1},
		{Key: "3", Value: []byte("1"), TimestampMilli: 1},
		{Key: "4", Value: []byte("1"), TimestampMilli: 1},
		{Key: "5", Value: []byte("1"), TimestampMilli: 1},
	}...))

	newIds, changedIds, remodedIds, err = fx1.Diff.Diff(ctx, fx2)
	require.NoError(t, err)
	assert.Len(t, newIds, 0)
	assert.Len(t, changedIds, 0)
	assert.Len(t, remodedIds, 0)
}

func newFixture(t testing.TB) *fixture {
	tmpDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	db, err := anystore.Open(ctx, filepath.Join(tmpDir, "test.db"), nil)
	require.NoError(t, err)
	coll, err := db.Collection(ctx, "test")
	require.NoError(t, err)
	kv, err := NewKeyValueStore(ctx, coll)
	require.NoError(t, err)
	fx := &fixture{
		keyValueStore: kv.(*keyValueStore),
		db:            db,
		coll:          coll,
	}
	t.Cleanup(func() {
		fx.finish(t)
		_ = os.RemoveAll(tmpDir)
	})
	return fx
}

type fixture struct {
	*keyValueStore
	db   anystore.DB
	coll anystore.Collection
}

func (fx *fixture) finish(t testing.TB) {
	assert.NoError(t, fx.db.Close())
}

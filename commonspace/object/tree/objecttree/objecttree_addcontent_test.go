package objecttree

import (
	"context"
	"errors"
	"testing"
	"time"

	anystore "github.com/anyproto/any-store"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
)

// failCommitStore fails the commit of every write transaction while failCommit is set.
type failCommitStore struct {
	anystore.DB
	failCommit bool
}

func (s *failCommitStore) WriteTx(ctx context.Context) (anystore.WriteTx, error) {
	tx, err := s.DB.WriteTx(ctx)
	if err != nil || !s.failCommit {
		return tx, err
	}
	return failCommitTx{tx}, nil
}

type failCommitTx struct {
	anystore.WriteTx
}

var errCommit = errors.New("commit failed")

func (tx failCommitTx) Commit() error {
	_ = tx.WriteTx.Rollback()
	return errCommit
}

type addContentFixture struct {
	aclList list.AclList
	keys    *accountdata.AccountKeys
	store   *failCommitStore
	storage *testStorage
	tree    ObjectTree
}

func newAddContentFixture(t *testing.T) *addContentFixture {
	aclList, keys := prepareAclList(t)
	root, err := CreateObjectTreeRoot(ObjectTreeCreatePayload{
		PrivKey:     keys.SignKey,
		ChangeType:  "changeType",
		SpaceId:     "spaceId",
		IsEncrypted: true,
	}, aclList)
	require.NoError(t, err)
	store := &failCommitStore{DB: createStore(ctx, t)}
	// the space storage keeps order ids unique per tree
	changesColl, err := store.Collection(ctx, CollName)
	require.NoError(t, err)
	require.NoError(t, changesColl.EnsureIndex(ctx, anystore.IndexInfo{
		Fields: []string{TreeKey, OrderKey},
		Unique: true,
	}))
	heads, err := headstorage.New(ctx, store)
	require.NoError(t, err)
	st, err := CreateStorage(ctx, root, heads, store)
	require.NoError(t, err)
	initTestAddSeq(st)
	fx := &addContentFixture{
		aclList: aclList,
		keys:    keys,
		store:   store,
		storage: &testStorage{Storage: st},
	}
	fx.rebuild(t)
	return fx
}

func (fx *addContentFixture) rebuild(t *testing.T) {
	tree, err := BuildObjectTree(fx.storage, fx.aclList)
	require.NoError(t, err)
	fx.tree = tree
}

func (fx *addContentFixture) content(isSnapshot bool) SignableChangeContent {
	return SignableChangeContent{
		Data:              []byte("some"),
		Key:               fx.keys.SignKey,
		IsSnapshot:        isSnapshot,
		ShouldBeEncrypted: true,
		DataType:          mockDataType,
	}
}

// build signs a change on top of prevIds without touching the tree.
func (fx *addContentFixture) build(t *testing.T, data string, prevIds ...string) *treechangeproto.RawTreeChangeWithId {
	ot := fx.tree.(*objectTree)
	_, raw, err := ot.changeBuilder.Build(BuilderContent{
		TreeHeadIds:    prevIds,
		AclHeadId:      fx.aclList.Head().Id,
		SnapshotBaseId: ot.tree.RootId(),
		Unencrypted:    true,
		PrivKey:        fx.keys.SignKey,
		Content:        []byte(data),
		Timestamp:      time.Now().Unix(),
		DataType:       mockDataType,
	})
	require.NoError(t, err)
	return raw
}

// requireStoredParents checks that every stored change has its parents stored.
func (fx *addContentFixture) requireStoredParents(t *testing.T) {
	var prevIds [][2]string
	require.NoError(t, fx.storage.GetAfterOrder(ctx, "", func(_ context.Context, ch StorageChange) (bool, error) {
		for _, prevId := range ch.PrevIds {
			prevIds = append(prevIds, [2]string{ch.Id, prevId})
		}
		return true, nil
	}))
	for _, ids := range prevIds {
		has, err := fx.storage.Has(ctx, ids[1])
		require.NoError(t, err)
		require.True(t, has, "%s is stored, its parent %s is not", ids[0], ids[1])
	}
}

func TestObjectTree_AddContentStorage(t *testing.T) {
	t.Run("refused change does not become a parent", func(t *testing.T) {
		fx := newAddContentFixture(t)
		first, err := fx.tree.AddContent(ctx, fx.content(false))
		require.NoError(t, err)

		fx.storage.errAdd = errors.New("error saving")
		_, err = fx.tree.AddContent(ctx, fx.content(false))
		require.ErrorIs(t, err, fx.storage.errAdd)
		require.Equal(t, first.Heads, fx.tree.Heads())
		require.Equal(t, first.Heads[0], fx.tree.(*objectTree).tree.lastIteratedHeadId)

		fx.storage.errAdd = nil
		res, err := fx.tree.AddContent(ctx, fx.content(false))
		require.NoError(t, err)
		require.Equal(t, first.Heads, res.Added[0].PrevIds)
		require.Equal(t, lexId.Next(first.Added[0].OrderId), res.Added[0].OrderId)
		fx.requireStoredParents(t)
		fx.rebuild(t)
		require.Equal(t, res.Heads, fx.tree.Heads())
	})

	t.Run("refused snapshot keeps the tree", func(t *testing.T) {
		fx := newAddContentFixture(t)
		first, err := fx.tree.AddContent(ctx, fx.content(false))
		require.NoError(t, err)
		rootId := fx.tree.Root().Id

		fx.storage.errAdd = errors.New("error saving")
		_, err = fx.tree.AddContent(ctx, fx.content(true))
		require.ErrorIs(t, err, fx.storage.errAdd)
		require.Equal(t, rootId, fx.tree.Root().Id)
		require.Equal(t, first.Heads, fx.tree.Heads())

		fx.storage.errAdd = nil
		res, err := fx.tree.AddContent(ctx, fx.content(true))
		require.NoError(t, err)
		require.Equal(t, res.Heads[0], fx.tree.Root().Id)
		stored, err := fx.storage.CommonSnapshot(ctx)
		require.NoError(t, err)
		require.Equal(t, res.Heads[0], stored)
		fx.requireStoredParents(t)
	})

	t.Run("validator sees the stored order id", func(t *testing.T) {
		fx := newAddContentFixture(t)
		var validated string
		res, err := fx.tree.AddContentWithValidator(ctx, fx.content(false), func(ch StorageChange) error {
			validated = ch.OrderId
			return nil
		})
		require.NoError(t, err)
		stored, err := fx.storage.Get(ctx, res.Added[0].Id)
		require.NoError(t, err)
		require.Equal(t, validated, stored.OrderId)
	})

	t.Run("storage returns the commit error", func(t *testing.T) {
		fx := newAddContentFixture(t)
		first, err := fx.tree.AddContent(ctx, fx.content(false))
		require.NoError(t, err)
		raw := fx.build(t, "refused", first.Heads...)
		refused := func() []StorageChange {
			return []StorageChange{{
				RawChange:  raw.RawChange,
				PrevIds:    first.Heads,
				Id:         raw.Id,
				OrderId:    lexId.Next(first.Added[0].OrderId),
				ChangeSize: len(raw.RawChange),
			}}
		}

		fx.store.failCommit = true
		st := fx.storage.Storage
		require.ErrorIs(t, st.AddAll(ctx, refused(), []string{raw.Id}, fx.tree.Id()), errCommit)
		require.ErrorIs(t, st.AddAllNoError(ctx, refused(), []string{raw.Id}, fx.tree.Id()), errCommit)

		fx.store.failCommit = false
		has, err := st.Has(ctx, raw.Id)
		require.NoError(t, err)
		require.False(t, has)
		heads, err := st.Heads(ctx)
		require.NoError(t, err)
		require.Equal(t, first.Heads, heads)
	})

	t.Run("failed commit is a refused change", func(t *testing.T) {
		fx := newAddContentFixture(t)
		first, err := fx.tree.AddContent(ctx, fx.content(false))
		require.NoError(t, err)

		fx.store.failCommit = true
		_, err = fx.tree.AddContent(ctx, fx.content(false))
		require.ErrorIs(t, err, errCommit)
		require.Equal(t, first.Heads, fx.tree.Heads())
		_, err = fx.tree.AddRawChanges(ctx, RawChangesPayload{
			NewHeads:   []string{"incoming"},
			RawChanges: []*treechangeproto.RawTreeChangeWithId{fx.build(t, "incoming", first.Heads...)},
		})
		require.ErrorIs(t, err, errCommit)
		require.Equal(t, first.Heads, fx.tree.Heads())

		fx.store.failCommit = false
		res, err := fx.tree.AddContent(ctx, fx.content(false))
		require.NoError(t, err)
		require.Equal(t, first.Heads, res.Added[0].PrevIds)
		fx.requireStoredParents(t)
		fx.rebuild(t)
		require.Equal(t, res.Heads, fx.tree.Heads())
	})
}

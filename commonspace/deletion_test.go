package commonspace

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	anystore "github.com/anyproto/any-store"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/spacepayloads"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/util/crypto"
)

func mockDeps() Deps {
	return Deps{
		TreeSyncer:     mockTreeSyncer{},
		SyncStatus:     syncstatus.NewNoOpSyncStatus(),
		recordVerifier: recordverifier.NewValidateFull(),
	}
}

func createTree(t *testing.T, ctx context.Context, spc Space, acc *accountdata.AccountKeys) string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	doc, err := spc.TreeBuilder().CreateTree(ctx, objecttree.ObjectTreeCreatePayload{
		PrivKey:     acc.SignKey,
		ChangeType:  "some",
		SpaceId:     spc.Id(),
		IsEncrypted: false,
		Seed:        bytes,
		Timestamp:   time.Now().Unix(),
	})
	require.NoError(t, err)
	tr, err := spc.TreeBuilder().PutTree(ctx, doc, nil)
	require.NoError(t, err)
	tr.Close()
	return tr.Id()
}

type storeSetter interface {
	SetStore(id string, store anystore.DB)
}

func TestSpaceDeleteIdsMarkDeleted(t *testing.T) {
	fx := newFixture(t)
	acc := fx.account.Account()
	rk := crypto.NewAES()
	privKey, _, _ := crypto.GenerateRandomEd25519KeyPair()
	ctx := context.Background()
	totalObjs := 100

	// creating space
	sp, err := fx.spaceService.CreateSpace(ctx, spacepayloads.SpaceCreatePayload{
		SigningKey:     acc.SignKey,
		SpaceType:      "type",
		ReadKey:        rk,
		MetadataKey:    privKey,
		ReplicationKey: 10,
		MasterKey:      acc.PeerKey,
	})
	require.NoError(t, err)
	require.NotNil(t, sp)

	// initializing space
	spc, err := fx.spaceService.NewSpace(ctx, sp, mockDeps())
	require.NoError(t, err)
	require.NotNil(t, spc)
	// adding space to tree manager
	fx.treeManager.space = spc
	err = spc.Init(ctx)
	require.NoError(t, err)
	close(fx.treeManager.waitLoad)

	anyStore := spc.Storage().AnyStore()
	newStore := objecttree.CopyStore(ctx, t, anyStore.(objecttree.TestStore), "store")

	var ids []string
	for i := 0; i < totalObjs; i++ {
		id := createTree(t, ctx, spc, acc)
		ids = append(ids, id)
	}
	// deleting trees, this will prepare the document to have all the deletion changes
	for _, id := range ids {
		err = spc.DeleteTree(ctx, id)
		require.NoError(t, err)
	}
	settingsId := spc.Storage().StateStorage().SettingsId()
	settingsStorage, err := spc.Storage().TreeStorage(ctx, settingsId)
	require.NoError(t, err)
	var allChanges []objecttree.StorageChange
	err = settingsStorage.GetAfterOrder(ctx, "", func(ctx context.Context, change objecttree.StorageChange) (shouldContinue bool, err error) {
		rawCh := make([]byte, len(change.RawChange))
		copy(rawCh, change.RawChange)
		change.RawChange = rawCh
		allChanges = append(allChanges, change)
		return true, nil
	})
	require.NoError(t, err)
	heads, err := settingsStorage.Heads(ctx)
	require.NoError(t, err)
	commonSnapshot, err := settingsStorage.CommonSnapshot(ctx)
	require.NoError(t, err)
	newStorage, err := spacestorage.New(ctx, spc.Id(), newStore)
	require.NoError(t, err)
	newSettingsStorage, err := newStorage.TreeStorage(ctx, settingsId)
	require.NoError(t, err)
	err = newSettingsStorage.AddAllNoError(ctx, allChanges, heads, commonSnapshot)
	require.NoError(t, err)
	err = spc.Close()
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	storeSetter := fx.storageProvider.(storeSetter)
	storeSetter.SetStore(sp, newStore)
	spc, err = fx.spaceService.NewSpace(ctx, sp, mockDeps())
	require.NoError(t, err)
	require.NotNil(t, spc)
	waitTest := make(chan struct{})
	fx.treeManager.wait = true
	fx.treeManager.space = spc
	fx.treeManager.condFunc = func() {
		if len(fx.treeManager.markedIds) == len(ids) {
			close(waitTest)
		}
	}
	fx.treeManager.waitLoad = make(chan struct{})
	fx.treeManager.deletedIds = nil
	fx.treeManager.markedIds = nil
	err = spc.Init(ctx)
	require.NoError(t, err)
	close(fx.treeManager.waitLoad)

	<-waitTest
	require.Equal(t, len(ids), len(fx.treeManager.markedIds))
	require.Zero(t, len(fx.treeManager.deletedIds))
}

func TestSpaceDeleteIds(t *testing.T) {
	fx := newFixture(t)
	acc := fx.account.Account()
	rk := crypto.NewAES()
	privKey, _, _ := crypto.GenerateRandomEd25519KeyPair()
	ctx := context.Background()
	totalObjs := 100

	// creating space
	sp, err := fx.spaceService.CreateSpace(ctx, spacepayloads.SpaceCreatePayload{
		SigningKey:     acc.SignKey,
		SpaceType:      "type",
		ReadKey:        rk,
		MetadataKey:    privKey,
		ReplicationKey: 10,
		MasterKey:      acc.PeerKey,
	})
	require.NoError(t, err)
	require.NotNil(t, sp)

	// initializing space
	spc, err := fx.spaceService.NewSpace(ctx, sp, mockDeps())
	require.NoError(t, err)
	require.NotNil(t, spc)
	// adding space to tree manager
	fx.treeManager.space = spc
	err = spc.Init(ctx)
	require.NoError(t, err)
	close(fx.treeManager.waitLoad)

	var ids []string
	for i := 0; i < totalObjs; i++ {
		id := createTree(t, ctx, spc, acc)
		ids = append(ids, id)
	}
	// copying storage, so we will have the same contents, except for empty trees
	anyStore := spc.Storage().AnyStore()
	newStore := objecttree.CopyStore(ctx, t, anyStore.(objecttree.TestStore), "store")
	// deleting trees, this will prepare the document to have all the deletion changes
	for _, id := range ids {
		err = spc.DeleteTree(ctx, id)
		require.NoError(t, err)
	}
	settingsId := spc.Storage().StateStorage().SettingsId()
	settingsStorage, err := spc.Storage().TreeStorage(ctx, settingsId)
	require.NoError(t, err)
	var allChanges []objecttree.StorageChange
	err = settingsStorage.GetAfterOrder(ctx, "", func(ctx context.Context, change objecttree.StorageChange) (shouldContinue bool, err error) {
		rawCh := make([]byte, len(change.RawChange))
		copy(rawCh, change.RawChange)
		change.RawChange = rawCh
		allChanges = append(allChanges, change)
		return true, nil
	})
	require.NoError(t, err)
	heads, err := settingsStorage.Heads(ctx)
	require.NoError(t, err)
	commonSnapshot, err := settingsStorage.CommonSnapshot(ctx)
	require.NoError(t, err)
	newStorage, err := spacestorage.New(ctx, spc.Id(), newStore)
	require.NoError(t, err)
	newSettingsStorage, err := newStorage.TreeStorage(ctx, settingsId)
	require.NoError(t, err)
	err = newSettingsStorage.AddAllNoError(ctx, allChanges, heads, commonSnapshot)
	require.NoError(t, err)
	err = spc.Close()
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	storeSetter := fx.storageProvider.(storeSetter)
	storeSetter.SetStore(sp, newStore)
	spc, err = fx.spaceService.NewSpace(ctx, sp, mockDeps())
	require.NoError(t, err)
	require.NotNil(t, spc)
	waitTest := make(chan struct{})
	fx.treeManager.wait = true
	fx.treeManager.space = spc
	fx.treeManager.waitLoad = make(chan struct{})
	fx.treeManager.deletedIds = nil
	fx.treeManager.markedIds = nil
	fx.treeManager.condFunc = func() {
		if len(fx.treeManager.deletedIds) == len(ids) {
			close(waitTest)
		}
	}
	err = spc.Init(ctx)
	require.NoError(t, err)
	close(fx.treeManager.waitLoad)
	// waiting until everything is deleted
	<-waitTest
	require.Equal(t, len(ids), len(fx.treeManager.deletedIds))
}

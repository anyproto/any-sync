package commonspace

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/settings"
	"github.com/anyproto/any-sync/commonspace/settings/settingsstate"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/util/crypto"
)

func addIncorrectSnapshot(settingsObject settings.SettingsObject, acc *accountdata.AccountKeys, partialIds map[string]struct{}, newId string) (err error) {
	factory := settingsstate.NewChangeFactory()
	bytes, err := factory.CreateObjectDeleteChange(newId, &settingsstate.State{DeletedIds: partialIds}, true)
	if err != nil {
		return
	}
	ch, err := settingsObject.PrepareChange(objecttree.SignableChangeContent{
		Data:        bytes,
		Key:         acc.SignKey,
		IsSnapshot:  true,
		IsEncrypted: false,
		Timestamp:   time.Now().Unix(),
	})
	if err != nil {
		return
	}
	res, err := settingsObject.AddRawChanges(context.Background(), objecttree.RawChangesPayload{
		NewHeads:   []string{ch.Id},
		RawChanges: []*treechangeproto.RawTreeChangeWithId{ch},
	})
	if err != nil {
		return
	}
	if res.Mode != objecttree.Rebuild {
		return fmt.Errorf("incorrect mode: %d", res.Mode)
	}
	return
}

func TestSpaceDeleteIds(t *testing.T) {
	fx := newFixture(t)
	acc := fx.account.Account()
	rk := crypto.NewAES()
	privKey, _, _ := crypto.GenerateRandomEd25519KeyPair()
	ctx := context.Background()
	totalObjs := 1500

	// creating space
	sp, err := fx.spaceService.CreateSpace(ctx, SpaceCreatePayload{
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
	spc, err := fx.spaceService.NewSpace(ctx, sp, Deps{TreeSyncer: mockTreeSyncer{}, SyncStatus: syncstatus.NewNoOpSyncStatus(), PeerStatus: mockPeerStatus{}})
	require.NoError(t, err)
	require.NotNil(t, spc)
	// adding space to tree manager
	fx.treeManager.space = spc
	err = spc.Init(ctx)
	require.NoError(t, err)
	close(fx.treeManager.waitLoad)

	var ids []string
	for i := 0; i < totalObjs; i++ {
		// creating a tree
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
		ids = append(ids, tr.Id())
		tr.Close()
	}
	// deleting trees
	for _, id := range ids {
		err = spc.DeleteTree(ctx, id)
		require.NoError(t, err)
	}
	time.Sleep(3 * time.Second)
	spc.Close()
	require.Equal(t, len(ids), len(fx.treeManager.deletedIds))
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

func TestSpaceDeleteIdsIncorrectSnapshot(t *testing.T) {
	fx := newFixture(t)
	acc := fx.account.Account()
	rk := crypto.NewAES()
	privKey, _, _ := crypto.GenerateRandomEd25519KeyPair()
	ctx := context.Background()
	totalObjs := 1500
	partialObjs := 300

	// creating space
	sp, err := fx.spaceService.CreateSpace(ctx, SpaceCreatePayload{
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
	spc, err := fx.spaceService.NewSpace(ctx, sp, Deps{TreeSyncer: mockTreeSyncer{}, SyncStatus: syncstatus.NewNoOpSyncStatus(), PeerStatus: mockPeerStatus{}})
	require.NoError(t, err)
	require.NotNil(t, spc)
	// adding space to tree manager
	fx.treeManager.space = spc
	err = spc.Init(ctx)
	close(fx.treeManager.waitLoad)
	require.NoError(t, err)

	settingsObject := spc.(*space).app.MustComponent(settings.CName).(settings.Settings).SettingsObject()
	var ids []string
	for i := 0; i < totalObjs; i++ {
		id := createTree(t, ctx, spc, acc)
		ids = append(ids, id)
	}
	// copying storage, so we will have all the trees locally
	inmemory := spc.Storage().(*commonStorage).SpaceStorage.(*spacestorage.InMemorySpaceStorage)
	storageCopy := inmemory.CopyStorage()
	treesCopy := inmemory.AllTrees()

	// deleting trees
	for _, id := range ids {
		err = spc.DeleteTree(ctx, id)
		require.NoError(t, err)
	}
	mapIds := map[string]struct{}{}
	for _, id := range ids[:partialObjs] {
		mapIds[id] = struct{}{}
	}
	// adding snapshot that breaks the state
	err = addIncorrectSnapshot(settingsObject, acc, mapIds, ids[partialObjs])
	require.NoError(t, err)
	// copying the contents of the settings tree
	treesCopy[settingsObject.Id()] = settingsObject.Storage()
	storageCopy.SetTrees(treesCopy)
	spc.Close()
	time.Sleep(100 * time.Millisecond)
	// now we replace the storage, so the trees are back, but the settings object says that they are deleted
	fx.storageProvider.(*spacestorage.InMemorySpaceStorageProvider).SetStorage(storageCopy)

	spc, err = fx.spaceService.NewSpace(ctx, sp, Deps{TreeSyncer: mockTreeSyncer{}, SyncStatus: syncstatus.NewNoOpSyncStatus(), PeerStatus: mockPeerStatus{}})
	require.NoError(t, err)
	require.NotNil(t, spc)
	fx.treeManager.waitLoad = make(chan struct{})
	fx.treeManager.space = spc
	fx.treeManager.deletedIds = nil
	err = spc.Init(ctx)
	require.NoError(t, err)
	close(fx.treeManager.waitLoad)

	// waiting until everything is deleted
	time.Sleep(3 * time.Second)
	require.Equal(t, len(ids), len(fx.treeManager.deletedIds))

	// checking that new snapshot will contain all the changes
	settingsObject = spc.(*space).app.MustComponent(settings.CName).(settings.Settings).SettingsObject()
	settings.DoSnapshot = func(treeLen int) bool {
		return true
	}
	id := createTree(t, ctx, spc, acc)
	err = spc.DeleteTree(ctx, id)
	require.NoError(t, err)
	delIds := settingsObject.Root().Model.(*spacesyncproto.SettingsData).Snapshot.DeletedIds
	require.Equal(t, totalObjs+1, len(delIds))
}

func TestSpaceDeleteIdsMarkDeleted(t *testing.T) {
	fx := newFixture(t)
	acc := fx.account.Account()
	rk := crypto.NewAES()
	privKey, _, _ := crypto.GenerateRandomEd25519KeyPair()
	ctx := context.Background()
	totalObjs := 1500

	// creating space
	sp, err := fx.spaceService.CreateSpace(ctx, SpaceCreatePayload{
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
	spc, err := fx.spaceService.NewSpace(ctx, sp, Deps{TreeSyncer: mockTreeSyncer{}, SyncStatus: syncstatus.NewNoOpSyncStatus(), PeerStatus: mockPeerStatus{}})
	require.NoError(t, err)
	require.NotNil(t, spc)
	// adding space to tree manager
	fx.treeManager.space = spc
	err = spc.Init(ctx)
	require.NoError(t, err)
	close(fx.treeManager.waitLoad)

	settingsObject := spc.(*space).app.MustComponent(settings.CName).(settings.Settings).SettingsObject()
	var ids []string
	for i := 0; i < totalObjs; i++ {
		id := createTree(t, ctx, spc, acc)
		ids = append(ids, id)
	}
	// copying storage, so we will have the same contents, except for empty trees
	inmemory := spc.Storage().(*commonStorage).SpaceStorage.(*spacestorage.InMemorySpaceStorage)
	storageCopy := inmemory.CopyStorage()

	// deleting trees, this will prepare the document to have all the deletion changes
	for _, id := range ids {
		err = spc.DeleteTree(ctx, id)
		require.NoError(t, err)
	}
	treesMap := map[string]treestorage.TreeStorage{}
	// copying the contents of the settings tree
	treesMap[settingsObject.Id()] = settingsObject.Storage()
	storageCopy.SetTrees(treesMap)
	spc.Close()
	time.Sleep(100 * time.Millisecond)
	// now we replace the storage, so the trees are back, but the settings object says that they are deleted
	fx.storageProvider.(*spacestorage.InMemorySpaceStorageProvider).SetStorage(storageCopy)

	spc, err = fx.spaceService.NewSpace(ctx, sp, Deps{TreeSyncer: mockTreeSyncer{}, SyncStatus: syncstatus.NewNoOpSyncStatus(), PeerStatus: mockPeerStatus{}})
	require.NoError(t, err)
	require.NotNil(t, spc)
	fx.treeManager.space = spc
	fx.treeManager.waitLoad = make(chan struct{})
	fx.treeManager.deletedIds = nil
	fx.treeManager.markedIds = nil
	err = spc.Init(ctx)
	require.NoError(t, err)
	close(fx.treeManager.waitLoad)

	// waiting until everything is deleted
	time.Sleep(3 * time.Second)
	require.Equal(t, len(ids), len(fx.treeManager.markedIds))
	require.Zero(t, len(fx.treeManager.deletedIds))
}

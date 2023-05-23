package commonspace

import (
	"context"
	"fmt"
	"github.com/anytypeio/any-sync/commonspace/object/accountdata"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/settings"
	"github.com/anytypeio/any-sync/commonspace/settings/settingsstate"
	"github.com/anytypeio/any-sync/commonspace/spacestorage"
	"github.com/anytypeio/any-sync/util/crypto"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
	"time"
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
	ctx := context.Background()
	totalObjs := 3000

	// creating space
	sp, err := fx.spaceService.CreateSpace(ctx, SpaceCreatePayload{
		SigningKey:     acc.SignKey,
		SpaceType:      "type",
		ReadKey:        rk.Bytes(),
		ReplicationKey: 10,
		MasterKey:      acc.PeerKey,
	})
	require.NoError(t, err)
	require.NotNil(t, sp)

	// initializing space
	spc, err := fx.spaceService.NewSpace(ctx, sp)
	require.NoError(t, err)
	require.NotNil(t, spc)
	err = spc.Init(ctx)
	require.NoError(t, err)
	// adding space to tree manager
	fx.treeManager.space = spc

	var ids []string
	for i := 0; i < totalObjs; i++ {
		// creating a tree
		bytes := make([]byte, 32)
		rand.Read(bytes)
		doc, err := spc.CreateTree(ctx, objecttree.ObjectTreeCreatePayload{
			PrivKey:     acc.SignKey,
			ChangeType:  "some",
			SpaceId:     spc.Id(),
			IsEncrypted: false,
			Seed:        bytes,
			Timestamp:   time.Now().Unix(),
		})
		require.NoError(t, err)
		tr, err := spc.PutTree(ctx, doc, nil)
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

func TestSpaceDeleteIdsIncorrectSnapshot(t *testing.T) {
	fx := newFixture(t)
	acc := fx.account.Account()
	rk := crypto.NewAES()
	ctx := context.Background()
	totalObjs := 3000
	partialObjs := 300

	// creating space
	sp, err := fx.spaceService.CreateSpace(ctx, SpaceCreatePayload{
		SigningKey:     acc.SignKey,
		SpaceType:      "type",
		ReadKey:        rk.Bytes(),
		ReplicationKey: 10,
		MasterKey:      acc.PeerKey,
	})
	require.NoError(t, err)
	require.NotNil(t, sp)

	// initializing space
	spc, err := fx.spaceService.NewSpace(ctx, sp)
	require.NoError(t, err)
	require.NotNil(t, spc)
	err = spc.Init(ctx)
	require.NoError(t, err)
	// adding space to tree manager
	fx.treeManager.space = spc

	settingsObject := spc.(*space).settingsObject
	var ids []string
	for i := 0; i < totalObjs; i++ {
		// creating a tree
		bytes := make([]byte, 32)
		rand.Read(bytes)
		doc, err := spc.CreateTree(ctx, objecttree.ObjectTreeCreatePayload{
			PrivKey:     acc.SignKey,
			ChangeType:  "some",
			SpaceId:     spc.Id(),
			IsEncrypted: false,
			Seed:        bytes,
			Timestamp:   time.Now().Unix(),
		})
		require.NoError(t, err)
		tr, err := spc.PutTree(ctx, doc, nil)
		require.NoError(t, err)
		ids = append(ids, tr.Id())
		tr.Close()
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

	spc, err = fx.spaceService.NewSpace(ctx, sp)
	require.NoError(t, err)
	require.NotNil(t, spc)
	err = spc.Init(ctx)
	require.NoError(t, err)
	fx.treeManager.space = spc
	fx.treeManager.deletedIds = nil

	// waiting until everything is deleted
	time.Sleep(3 * time.Second)
	require.Equal(t, len(ids), len(fx.treeManager.deletedIds))
}

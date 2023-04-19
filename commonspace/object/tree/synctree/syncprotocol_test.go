package synctree

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/object/accountdata"
	"github.com/anytypeio/any-sync/commonspace/object/acl/list"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
	"github.com/anytypeio/any-sync/util/slice"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
	"time"
)

func TestEmptyClientGetsFullHistory(t *testing.T) {
	treeId := "treeId"
	spaceId := "spaceId"
	keys, err := accountdata.NewRandom()
	require.NoError(t, err)
	aclList, err := list.NewTestDerivedAcl(spaceId, keys)
	require.NoError(t, err)
	storage := createStorage(treeId, aclList)
	changeCreator := objecttree.NewMockChangeCreator()
	deps := fixtureDeps{
		aclList:     aclList,
		initStorage: storage.(*treestorage.InMemoryTreeStorage),
		connectionMap: map[string][]string{
			"peer1": []string{"peer2"},
			"peer2": []string{"peer1"},
		},
		emptyTrees: []string{"peer2"},
	}
	fx := newProcessFixture(t, spaceId, deps)
	fx.run(t)
	fx.handlers["peer1"].sendRawChanges(context.Background(), objecttree.RawChangesPayload{
		NewHeads: nil,
		RawChanges: []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("1", aclList.Id(), treeId, true, treeId),
		},
	})
	time.Sleep(100 * time.Millisecond)
	fx.stop()
	firstHeads := fx.handlers["peer1"].tree().Heads()
	secondHeads := fx.handlers["peer2"].tree().Heads()
	require.True(t, slice.SortedEquals(firstHeads, secondHeads))
	require.Equal(t, []string{"1"}, firstHeads)
	logMsgs := fx.log.batcher.GetAll()

	var fullResponseMsg msgDescription
	for _, msg := range logMsgs {
		descr := msg.description()
		if descr.name == "FullSyncResponse" {
			fullResponseMsg = descr
		}
	}
	// that means that we got not only the last snapshot, but also the first one
	require.Len(t, fullResponseMsg.changes, 2)
}

func TestRandomMerge(t *testing.T) {
	var (
		rnd      = rand.New(rand.NewSource(time.Now().Unix()))
		levels   = 20
		perLevel = 20
		rounds   = 10
	)
	for i := 0; i < rounds; i++ {
		testTreeMerge(t, levels, perLevel, func() bool {
			return true
		})
		testTreeMerge(t, levels, perLevel, func() bool {
			return false
		})
		testTreeMerge(t, levels, perLevel, func() bool {
			return rnd.Intn(10) > 8
		})
		levels += 2
	}
}

func testTreeMerge(t *testing.T, levels, perlevel int, isSnapshot func() bool) {
	treeId := "treeId"
	spaceId := "spaceId"
	keys, err := accountdata.NewRandom()
	require.NoError(t, err)
	aclList, err := list.NewTestDerivedAcl(spaceId, keys)
	storage := createStorage(treeId, aclList)
	changeCreator := objecttree.NewMockChangeCreator()
	params := genParams{
		prefix:     "peer1",
		aclId:      aclList.Id(),
		startIdx:   0,
		levels:     levels,
		perLevel:   perlevel,
		snapshotId: treeId,
		prevHeads:  []string{treeId},
		isSnapshot: isSnapshot,
	}
	initialRes := genChanges(changeCreator, params)
	err = storage.TransactionAdd(initialRes.changes, initialRes.heads)
	require.NoError(t, err)
	deps := fixtureDeps{
		aclList:     aclList,
		initStorage: storage.(*treestorage.InMemoryTreeStorage),
		connectionMap: map[string][]string{
			"peer1": []string{"node1"},
			"peer2": []string{"node1"},
			"node1": []string{"peer1", "peer2"},
		},
		emptyTrees: []string{"peer2", "node1"},
	}
	fx := newProcessFixture(t, spaceId, deps)
	fx.run(t)
	fx.handlers["peer1"].sendRawChanges(context.Background(), objecttree.RawChangesPayload{
		NewHeads:   initialRes.heads,
		RawChanges: initialRes.changes,
	})
	time.Sleep(50 * time.Millisecond)
	firstHeads := fx.handlers["peer1"].tree().Heads()
	secondHeads := fx.handlers["peer2"].tree().Heads()
	require.True(t, slice.UnsortedEquals(firstHeads, secondHeads))
	params = genParams{
		prefix:   "peer1",
		aclId:    aclList.Id(),
		startIdx: levels,
		levels:   levels,
		perLevel: perlevel,

		snapshotId: initialRes.snapshotId,
		prevHeads:  initialRes.heads,
		isSnapshot: isSnapshot,
	}
	peer1Res := genChanges(changeCreator, params)
	params.prefix = "peer2"
	peer2Res := genChanges(changeCreator, params)
	fx.handlers["peer1"].sendRawChanges(context.Background(), objecttree.RawChangesPayload{
		NewHeads:   peer1Res.heads,
		RawChanges: peer1Res.changes,
	})
	fx.handlers["peer2"].sendRawChanges(context.Background(), objecttree.RawChangesPayload{
		NewHeads:   peer2Res.heads,
		RawChanges: peer2Res.changes,
	})
	time.Sleep(50 * time.Millisecond)
	fx.stop()
	firstTree := fx.handlers["peer1"].tree()
	secondTree := fx.handlers["peer2"].tree()
	firstHeads = firstTree.Heads()
	secondHeads = secondTree.Heads()
	require.True(t, slice.UnsortedEquals(firstHeads, secondHeads))
	require.True(t, slice.UnsortedEquals(firstHeads, append(peer1Res.heads, peer2Res.heads...)))
	firstStorage := firstTree.Storage().(*treestorage.InMemoryTreeStorage)
	secondStorage := secondTree.Storage().(*treestorage.InMemoryTreeStorage)
	require.True(t, firstStorage.Equal(secondStorage))
}

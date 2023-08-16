package synctree

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/util/slice"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
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
		counter:     defaultOpCounter(),
		aclList:     aclList,
		initStorage: storage.(*treestorage.InMemoryTreeStorage),
		connectionMap: map[string][]string{
			"peer1": []string{"peer2"},
			"peer2": []string{"peer1"},
		},
		emptyTrees:  []string{"peer2"},
		treeBuilder: objecttree.BuildTestableTree,
	}
	fx := newProtocolFixture(t, spaceId, deps)
	fx.run(t)
	fx.handlers["peer1"].sendRawChanges(context.Background(), objecttree.RawChangesPayload{
		NewHeads: nil,
		RawChanges: []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("1", aclList.Id(), treeId, true, treeId),
		},
	})
	fx.counter.WaitIdle()
	fx.stop()
	firstHeads := fx.handlers["peer1"].tree().Heads()
	secondHeads := fx.handlers["peer2"].tree().Heads()
	require.True(t, slice.UnsortedEquals(firstHeads, secondHeads))
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

func TestTreeFuzzyMerge(t *testing.T) {
	testTreeFuzzyMerge(t, true)
	testTreeFuzzyMerge(t, false)
}

func testTreeFuzzyMerge(t *testing.T, withData bool) {
	var (
		rnd      = rand.New(rand.NewSource(time.Now().Unix()))
		levels   = 20
		perLevel = 20
		rounds   = 10
	)
	for i := 0; i < rounds; i++ {
		testTreeMerge(t, levels, perLevel, withData, func() bool {
			return true
		})
		testTreeMerge(t, levels, perLevel, withData, func() bool {
			return false
		})
		testTreeMerge(t, levels, perLevel, withData, func() bool {
			return rnd.Intn(10) > 8
		})
		levels += 2
	}
}

func testTreeMerge(t *testing.T, levels, perLevel int, hasData bool, isSnapshot func() bool) {
	treeId := "treeId"
	spaceId := "spaceId"
	keys, err := accountdata.NewRandom()
	require.NoError(t, err)
	aclList, err := list.NewTestDerivedAcl(spaceId, keys)
	storage := createStorage(treeId, aclList)
	changeCreator := objecttree.NewMockChangeCreator()
	builder := objecttree.BuildTestableTree
	if hasData {
		builder = objecttree.BuildEmptyDataTestableTree
	}
	params := genParams{
		prefix:     "peer1",
		aclId:      aclList.Id(),
		startIdx:   0,
		levels:     levels,
		perLevel:   perLevel,
		snapshotId: treeId,
		prevHeads:  []string{treeId},
		isSnapshot: isSnapshot,
		hasData:    hasData,
	}
	// generating initial tree
	initialRes := genChanges(changeCreator, params)
	err = storage.AddRawChangesSetHeads(initialRes.changes, initialRes.heads)
	require.NoError(t, err)
	deps := fixtureDeps{
		counter:     defaultOpCounter(),
		aclList:     aclList,
		initStorage: storage.(*treestorage.InMemoryTreeStorage),
		connectionMap: map[string][]string{
			"peer1": []string{"node1"},
			"peer2": []string{"node1"},
			"node1": []string{"peer1", "peer2"},
		},
		emptyTrees:  []string{"peer2", "node1"},
		treeBuilder: builder,
	}
	fx := newProtocolFixture(t, spaceId, deps)
	fx.run(t)
	fx.handlers["peer1"].sendRawChanges(context.Background(), objecttree.RawChangesPayload{
		NewHeads:   initialRes.heads,
		RawChanges: initialRes.changes,
	})
	fx.counter.WaitIdle()
	firstHeads := fx.handlers["peer1"].tree().Heads()
	secondHeads := fx.handlers["peer2"].tree().Heads()
	require.True(t, slice.UnsortedEquals(firstHeads, secondHeads))
	params = genParams{
		prefix:   "peer1",
		aclId:    aclList.Id(),
		startIdx: levels,
		levels:   levels,
		perLevel: perLevel,

		snapshotId: initialRes.snapshotId,
		prevHeads:  initialRes.heads,
		isSnapshot: isSnapshot,
		hasData:    hasData,
	}
	// generating different additions to the tree for different peers
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
	fx.counter.WaitIdle()
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
	if hasData {
		for _, ch := range secondStorage.Changes {
			if ch.Id == treeId {
				continue
			}
			unmarshallRaw := &treechangeproto.RawTreeChange{}
			proto.Unmarshal(ch.RawChange, unmarshallRaw)
			treeCh := &treechangeproto.TreeChange{}
			proto.Unmarshal(unmarshallRaw.Payload, treeCh)
			require.Equal(t, ch.Id, string(treeCh.ChangesData))
		}
	}
}

func TestTreeStorageHasExtraChanges(t *testing.T) {
	var (
		rnd      = rand.New(rand.NewSource(time.Now().Unix()))
		levels   = 20
		perLevel = 40
	)

	// simulating cases where one peer has some extra changes saved in storage
	// and checking that this will not break the sync
	t.Run("tree storage has extra simple", func(t *testing.T) {
		testTreeStorageHasExtra(t, levels, perLevel, false, func() bool {
			return false
		})
		testTreeStorageHasExtra(t, levels, perLevel, false, func() bool {
			return rnd.Intn(10) > 5
		})
		testTreeStorageHasExtra(t, levels, perLevel, true, func() bool {
			return false
		})
		testTreeStorageHasExtra(t, levels, perLevel, true, func() bool {
			return rnd.Intn(10) > 5
		})
	})
	t.Run("tree storage has extra three parts", func(t *testing.T) {
		testTreeStorageHasExtraThreeParts(t, levels, perLevel, false, func() bool {
			return false
		})
		testTreeStorageHasExtraThreeParts(t, levels, perLevel, false, func() bool {
			return rnd.Intn(10) > 5
		})
		testTreeStorageHasExtraThreeParts(t, levels, perLevel, true, func() bool {
			return false
		})
		testTreeStorageHasExtraThreeParts(t, levels, perLevel, true, func() bool {
			return rnd.Intn(10) > 5
		})
	})
}

func testTreeStorageHasExtra(t *testing.T, levels, perLevel int, hasData bool, isSnapshot func() bool) {
	treeId := "treeId"
	spaceId := "spaceId"
	keys, err := accountdata.NewRandom()
	require.NoError(t, err)
	aclList, err := list.NewTestDerivedAcl(spaceId, keys)
	storage := createStorage(treeId, aclList)
	changeCreator := objecttree.NewMockChangeCreator()
	builder := objecttree.BuildTestableTree
	if hasData {
		builder = objecttree.BuildEmptyDataTestableTree
	}
	params := genParams{
		prefix:     "peer1",
		aclId:      aclList.Id(),
		startIdx:   0,
		levels:     levels,
		perLevel:   perLevel,
		snapshotId: treeId,
		prevHeads:  []string{treeId},
		isSnapshot: isSnapshot,
		hasData:    hasData,
	}
	deps := fixtureDeps{
		counter:     defaultOpCounter(),
		aclList:     aclList,
		initStorage: storage.(*treestorage.InMemoryTreeStorage),
		connectionMap: map[string][]string{
			"peer1": []string{"peer2"},
			"peer2": []string{"peer1"},
		},
		treeBuilder: builder,
	}
	fx := newProtocolFixture(t, spaceId, deps)

	// generating initial tree
	initialRes := genChanges(changeCreator, params)
	fx.run(t)

	// adding some changes to store, but without updating heads
	store := fx.handlers["peer1"].tree().Storage().(*treestorage.InMemoryTreeStorage)
	oldHeads, _ := store.Heads()
	store.AddRawChangesSetHeads(initialRes.changes, oldHeads)

	// sending those changes to other peer
	fx.handlers["peer2"].sendRawChanges(context.Background(), objecttree.RawChangesPayload{
		NewHeads:   initialRes.heads,
		RawChanges: initialRes.changes,
	})
	fx.counter.WaitIdle()

	// here we want that the saved changes in storage should not affect the sync protocol
	firstHeads := fx.handlers["peer1"].tree().Heads()
	secondHeads := fx.handlers["peer2"].tree().Heads()
	require.True(t, slice.UnsortedEquals(firstHeads, secondHeads))
	require.True(t, slice.UnsortedEquals(initialRes.heads, firstHeads))
}

func testTreeStorageHasExtraThreeParts(t *testing.T, levels, perLevel int, hasData bool, isSnapshot func() bool) {
	treeId := "treeId"
	spaceId := "spaceId"
	keys, err := accountdata.NewRandom()
	require.NoError(t, err)
	aclList, err := list.NewTestDerivedAcl(spaceId, keys)
	storage := createStorage(treeId, aclList)
	changeCreator := objecttree.NewMockChangeCreator()
	builder := objecttree.BuildTestableTree
	if hasData {
		builder = objecttree.BuildEmptyDataTestableTree
	}
	params := genParams{
		prefix:     "peer1",
		aclId:      aclList.Id(),
		startIdx:   0,
		levels:     levels,
		perLevel:   perLevel,
		snapshotId: treeId,
		prevHeads:  []string{treeId},
		isSnapshot: isSnapshot,
		hasData:    hasData,
	}
	deps := fixtureDeps{
		counter:     defaultOpCounter(),
		aclList:     aclList,
		initStorage: storage.(*treestorage.InMemoryTreeStorage),
		connectionMap: map[string][]string{
			"peer1": []string{"peer2"},
			"peer2": []string{"peer1"},
		},
		treeBuilder: builder,
	}
	fx := newProtocolFixture(t, spaceId, deps)

	// generating parts
	firstPart := genChanges(changeCreator, params)
	params.startIdx = levels
	params.snapshotId = firstPart.snapshotId
	params.prevHeads = firstPart.heads
	secondPart := genChanges(changeCreator, params)
	params.startIdx = levels * 2
	params.snapshotId = secondPart.snapshotId
	params.prevHeads = secondPart.heads
	thirdPart := genChanges(changeCreator, params)

	// adding part1 to first peer and saving part2 and part3 in its storage
	res, _ := fx.handlers["peer1"].tree().AddRawChanges(context.Background(), objecttree.RawChangesPayload{
		NewHeads:   firstPart.heads,
		RawChanges: firstPart.changes,
	})
	require.True(t, slice.UnsortedEquals(res.Heads, firstPart.heads))
	store := fx.handlers["peer1"].tree().Storage().(*treestorage.InMemoryTreeStorage)
	oldHeads, _ := store.Heads()
	store.AddRawChangesSetHeads(secondPart.changes, oldHeads)
	store.AddRawChangesSetHeads(thirdPart.changes, oldHeads)

	var peer2Initial []*treechangeproto.RawTreeChangeWithId
	peer2Initial = append(peer2Initial, firstPart.changes...)
	peer2Initial = append(peer2Initial, secondPart.changes...)

	// adding part1 and part2 to second peer
	res, _ = fx.handlers["peer2"].tree().AddRawChanges(context.Background(), objecttree.RawChangesPayload{
		NewHeads:   secondPart.heads,
		RawChanges: peer2Initial,
	})
	require.True(t, slice.UnsortedEquals(res.Heads, secondPart.heads))
	fx.run(t)

	// sending part3 changes to other peer
	fx.handlers["peer2"].sendRawChanges(context.Background(), objecttree.RawChangesPayload{
		NewHeads:   thirdPart.heads,
		RawChanges: thirdPart.changes,
	})
	fx.counter.WaitIdle()
	firstHeads := fx.handlers["peer1"].tree().Heads()
	secondHeads := fx.handlers["peer2"].tree().Heads()
	require.True(t, slice.UnsortedEquals(firstHeads, secondHeads))
	require.True(t, slice.UnsortedEquals(thirdPart.heads, firstHeads))
}

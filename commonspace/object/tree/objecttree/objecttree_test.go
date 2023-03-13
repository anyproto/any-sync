package objecttree

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/object/acl/list"
	"github.com/anytypeio/any-sync/commonspace/object/acl/testutils/acllistbuilder"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

type testTreeContext struct {
	aclList       list.AclList
	treeStorage   treestorage.TreeStorage
	changeBuilder ChangeBuilder
	changeCreator *MockChangeCreator
	objTree       ObjectTree
}

func prepareAclList(t *testing.T) list.AclList {
	st, err := acllistbuilder.NewListStorageWithTestName("userjoinexample.yml")
	require.NoError(t, err, "building storage should not result in error")

	aclList, err := list.BuildAclList(st)
	require.NoError(t, err, "building acl list should be without error")

	return aclList
}

func prepareTreeDeps(aclList list.AclList) (*MockChangeCreator, objectTreeDeps) {
	changeCreator := &MockChangeCreator{}
	treeStorage := changeCreator.CreateNewTreeStorage("0", aclList.Head().Id)
	root, _ := treeStorage.Root()
	changeBuilder := &nonVerifiableChangeBuilder{
		ChangeBuilder: NewChangeBuilder(nil, root),
	}
	deps := objectTreeDeps{
		changeBuilder:   changeBuilder,
		treeBuilder:     newTreeBuilder(treeStorage, changeBuilder),
		treeStorage:     treeStorage,
		rawChangeLoader: newRawChangeLoader(treeStorage, changeBuilder),
		validator:       &noOpTreeValidator{},
		aclList:         aclList,
	}
	return changeCreator, deps
}

func prepareTreeContext(t *testing.T, aclList list.AclList) testTreeContext {
	changeCreator := &MockChangeCreator{}
	treeStorage := changeCreator.CreateNewTreeStorage("0", aclList.Head().Id)
	root, _ := treeStorage.Root()
	changeBuilder := &nonVerifiableChangeBuilder{
		ChangeBuilder: NewChangeBuilder(nil, root),
	}
	deps := objectTreeDeps{
		changeBuilder:   changeBuilder,
		treeBuilder:     newTreeBuilder(treeStorage, changeBuilder),
		treeStorage:     treeStorage,
		rawChangeLoader: newRawChangeLoader(treeStorage, changeBuilder),
		validator:       &noOpTreeValidator{},
		aclList:         aclList,
	}

	// check build
	objTree, err := buildObjectTree(deps)
	require.NoError(t, err, "building tree should be without error")

	// check tree iterate
	var iterChangesId []string
	err = objTree.IterateRoot(nil, func(change *Change) bool {
		iterChangesId = append(iterChangesId, change.Id)
		return true
	})
	require.NoError(t, err, "iterate should be without error")
	assert.Equal(t, []string{"0"}, iterChangesId)
	return testTreeContext{
		aclList:       aclList,
		treeStorage:   treeStorage,
		changeBuilder: changeBuilder,
		changeCreator: changeCreator,
		objTree:       objTree,
	}
}

func TestObjectTree(t *testing.T) {
	aclList := prepareAclList(t)

	t.Run("add simple", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		treeStorage := ctx.treeStorage
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree

		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("1", aclList.Head().Id, "0", false, "0"),
			changeCreator.CreateRaw("2", aclList.Head().Id, "0", false, "1"),
		}
		payload := RawChangesPayload{
			NewHeads:   []string{rawChanges[len(rawChanges)-1].Id},
			RawChanges: rawChanges,
		}
		res, err := objTree.AddRawChanges(context.Background(), payload)
		require.NoError(t, err, "adding changes should be without error")

		// check result
		assert.Equal(t, []string{"0"}, res.OldHeads)
		assert.Equal(t, []string{"2"}, res.Heads)
		assert.Equal(t, len(rawChanges), len(res.Added))
		assert.Equal(t, Append, res.Mode)

		// check tree heads
		assert.Equal(t, []string{"2"}, objTree.Heads())

		// check tree iterate
		var iterChangesId []string
		err = objTree.IterateRoot(nil, func(change *Change) bool {
			iterChangesId = append(iterChangesId, change.Id)
			return true
		})
		require.NoError(t, err, "iterate should be without error")
		assert.Equal(t, []string{"0", "1", "2"}, iterChangesId)

		// check storage
		heads, _ := treeStorage.Heads()
		assert.Equal(t, []string{"2"}, heads)

		for _, ch := range rawChanges {
			raw, err := treeStorage.GetRawChange(context.Background(), ch.Id)
			assert.NoError(t, err, "storage should have all the changes")
			assert.Equal(t, ch, raw, "the changes in the storage should be the same")
		}
	})

	t.Run("add no new changes", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree

		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("0", aclList.Head().Id, "", true, ""),
		}
		payload := RawChangesPayload{
			NewHeads:   []string{rawChanges[len(rawChanges)-1].Id},
			RawChanges: rawChanges,
		}
		res, err := objTree.AddRawChanges(context.Background(), payload)
		require.NoError(t, err, "adding changes should be without error")

		// check result
		assert.Equal(t, []string{"0"}, res.OldHeads)
		assert.Equal(t, []string{"0"}, res.Heads)
		assert.Equal(t, 0, len(res.Added))

		// check tree heads
		assert.Equal(t, []string{"0"}, objTree.Heads())
	})

	t.Run("add unattachable changes", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree

		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("2", aclList.Head().Id, "0", false, "1"),
		}
		payload := RawChangesPayload{
			NewHeads:   []string{rawChanges[len(rawChanges)-1].Id},
			RawChanges: rawChanges,
		}
		res, err := objTree.AddRawChanges(context.Background(), payload)
		require.NoError(t, err, "adding changes should be without error")

		// check result
		assert.Equal(t, []string{"0"}, res.OldHeads)
		assert.Equal(t, []string{"0"}, res.Heads)
		assert.Equal(t, 0, len(res.Added))
		assert.Equal(t, Nothing, res.Mode)

		// check tree heads
		assert.Equal(t, []string{"0"}, objTree.Heads())
	})

	t.Run("add new snapshot simple", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		treeStorage := ctx.treeStorage
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree

		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("1", aclList.Head().Id, "0", false, "0"),
			changeCreator.CreateRaw("2", aclList.Head().Id, "0", false, "1"),
			changeCreator.CreateRaw("3", aclList.Head().Id, "0", true, "2"),
			changeCreator.CreateRaw("4", aclList.Head().Id, "3", false, "3"),
		}
		payload := RawChangesPayload{
			NewHeads:   []string{rawChanges[len(rawChanges)-1].Id},
			RawChanges: rawChanges,
		}

		res, err := objTree.AddRawChanges(context.Background(), payload)
		require.NoError(t, err, "adding changes should be without error")

		// check result
		assert.Equal(t, []string{"0"}, res.OldHeads)
		assert.Equal(t, []string{"4"}, res.Heads)
		assert.Equal(t, len(rawChanges), len(res.Added))
		// here we have rebuild, because we reduced tree to new snapshot
		assert.Equal(t, Rebuild, res.Mode)

		// check tree heads
		assert.Equal(t, []string{"4"}, objTree.Heads())

		// check tree iterate
		var iterChangesId []string
		err = objTree.IterateRoot(nil, func(change *Change) bool {
			iterChangesId = append(iterChangesId, change.Id)
			return true
		})
		require.NoError(t, err, "iterate should be without error")
		assert.Equal(t, []string{"3", "4"}, iterChangesId)
		assert.Equal(t, "3", objTree.Root().Id)

		// check storage
		heads, _ := treeStorage.Heads()
		assert.Equal(t, []string{"4"}, heads)

		for _, ch := range rawChanges {
			raw, err := treeStorage.GetRawChange(context.Background(), ch.Id)
			assert.NoError(t, err, "storage should have all the changes")
			assert.Equal(t, ch, raw, "the changes in the storage should be the same")
		}
	})

	t.Run("snapshot path", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree

		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("1", aclList.Head().Id, "0", false, "0"),
			changeCreator.CreateRaw("2", aclList.Head().Id, "0", false, "1"),
			changeCreator.CreateRaw("3", aclList.Head().Id, "0", true, "2"),
		}
		payload := RawChangesPayload{
			NewHeads:   []string{rawChanges[len(rawChanges)-1].Id},
			RawChanges: rawChanges,
		}

		_, err := objTree.AddRawChanges(context.Background(), payload)
		require.NoError(t, err, "adding changes should be without error")

		snapshotPath := objTree.SnapshotPath()
		assert.Equal(t, []string{"3", "0"}, snapshotPath)

		assert.Equal(t, true, objTree.(*objectTree).snapshotPathIsActual())
	})

	t.Run("changes from tree after common snapshot complex", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree

		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("1", aclList.Head().Id, "0", false, "0"),
			changeCreator.CreateRaw("2", aclList.Head().Id, "0", false, "1"),
			changeCreator.CreateRaw("3", aclList.Head().Id, "0", true, "2"),
			changeCreator.CreateRaw("4", aclList.Head().Id, "0", false, "2"),
			changeCreator.CreateRaw("5", aclList.Head().Id, "0", false, "1"),
			changeCreator.CreateRaw("6", aclList.Head().Id, "0", false, "3", "4", "5"),
		}

		payload := RawChangesPayload{
			NewHeads:   []string{rawChanges[len(rawChanges)-1].Id},
			RawChanges: rawChanges,
		}

		_, err := objTree.AddRawChanges(context.Background(), payload)
		require.NoError(t, err, "adding changes should be without error")
		require.Equal(t, "0", objTree.Root().Id)

		t.Run("all changes from tree", func(t *testing.T) {
			changes, err := objTree.ChangesAfterCommonSnapshot([]string{"3", "0"}, []string{})
			require.NoError(t, err, "changes after common snapshot should be without error")

			changeIds := make(map[string]struct{})
			for _, ch := range changes {
				changeIds[ch.Id] = struct{}{}
			}

			for _, raw := range rawChanges {
				_, ok := changeIds[raw.Id]
				assert.Equal(t, true, ok)
			}
			_, ok := changeIds["0"]
			assert.Equal(t, true, ok)
		})

		t.Run("changes from tree after 1", func(t *testing.T) {
			changes, err := objTree.ChangesAfterCommonSnapshot([]string{"3", "0"}, []string{"1"})
			require.NoError(t, err, "changes after common snapshot should be without error")

			changeIds := make(map[string]struct{})
			for _, ch := range changes {
				changeIds[ch.Id] = struct{}{}
			}

			for _, id := range []string{"2", "3", "4", "5", "6"} {
				_, ok := changeIds[id]
				assert.Equal(t, true, ok)
			}
			for _, id := range []string{"0", "1"} {
				_, ok := changeIds[id]
				assert.Equal(t, false, ok)
			}
		})

		t.Run("changes from tree after 5", func(t *testing.T) {
			changes, err := objTree.ChangesAfterCommonSnapshot([]string{"3", "0"}, []string{"5"})
			require.NoError(t, err, "changes after common snapshot should be without error")

			changeIds := make(map[string]struct{})
			for _, ch := range changes {
				changeIds[ch.Id] = struct{}{}
			}

			for _, id := range []string{"2", "3", "4", "6"} {
				_, ok := changeIds[id]
				assert.Equal(t, true, ok)
			}
			for _, id := range []string{"0", "1", "5"} {
				_, ok := changeIds[id]
				assert.Equal(t, false, ok)
			}
		})
	})

	t.Run("changes after common snapshot db complex", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree

		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("1", aclList.Head().Id, "0", false, "0"),
			changeCreator.CreateRaw("2", aclList.Head().Id, "0", false, "1"),
			changeCreator.CreateRaw("3", aclList.Head().Id, "0", true, "2"),
			changeCreator.CreateRaw("4", aclList.Head().Id, "0", false, "2"),
			changeCreator.CreateRaw("5", aclList.Head().Id, "0", false, "1"),
			// main difference from tree example
			changeCreator.CreateRaw("6", aclList.Head().Id, "0", true, "3", "4", "5"),
		}

		payload := RawChangesPayload{
			NewHeads:   []string{rawChanges[len(rawChanges)-1].Id},
			RawChanges: rawChanges,
		}

		_, err := objTree.AddRawChanges(context.Background(), payload)
		require.NoError(t, err, "adding changes should be without error")
		require.Equal(t, "6", objTree.Root().Id)

		t.Run("all changes from db", func(t *testing.T) {
			changes, err := objTree.ChangesAfterCommonSnapshot([]string{"3", "0"}, []string{})
			require.NoError(t, err, "changes after common snapshot should be without error")

			changeIds := make(map[string]struct{})
			for _, ch := range changes {
				changeIds[ch.Id] = struct{}{}
			}

			for _, raw := range rawChanges {
				_, ok := changeIds[raw.Id]
				assert.Equal(t, true, ok)
			}
			_, ok := changeIds["0"]
			assert.Equal(t, true, ok)
		})

		t.Run("changes from tree db 1", func(t *testing.T) {
			changes, err := objTree.ChangesAfterCommonSnapshot([]string{"3", "0"}, []string{"1"})
			require.NoError(t, err, "changes after common snapshot should be without error")

			changeIds := make(map[string]struct{})
			for _, ch := range changes {
				changeIds[ch.Id] = struct{}{}
			}

			for _, id := range []string{"2", "3", "4", "5", "6"} {
				_, ok := changeIds[id]
				assert.Equal(t, true, ok)
			}
			for _, id := range []string{"0", "1"} {
				_, ok := changeIds[id]
				assert.Equal(t, false, ok)
			}
		})

		t.Run("changes from tree db 5", func(t *testing.T) {
			changes, err := objTree.ChangesAfterCommonSnapshot([]string{"3", "0"}, []string{"5"})
			require.NoError(t, err, "changes after common snapshot should be without error")

			changeIds := make(map[string]struct{})
			for _, ch := range changes {
				changeIds[ch.Id] = struct{}{}
			}

			for _, id := range []string{"2", "3", "4", "6"} {
				_, ok := changeIds[id]
				assert.Equal(t, true, ok)
			}
			for _, id := range []string{"0", "1", "5"} {
				_, ok := changeIds[id]
				assert.Equal(t, false, ok)
			}
		})
	})

	t.Run("add new changes related to previous snapshot", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		treeStorage := ctx.treeStorage
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree

		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("1", aclList.Head().Id, "0", false, "0"),
			changeCreator.CreateRaw("2", aclList.Head().Id, "0", false, "1"),
			changeCreator.CreateRaw("3", aclList.Head().Id, "0", true, "2"),
		}
		payload := RawChangesPayload{
			NewHeads:   []string{rawChanges[len(rawChanges)-1].Id},
			RawChanges: rawChanges,
		}

		res, err := objTree.AddRawChanges(context.Background(), payload)
		require.NoError(t, err, "adding changes should be without error")
		require.Equal(t, "3", objTree.Root().Id)

		rawChanges = []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("4", aclList.Head().Id, "0", false, "2"),
			changeCreator.CreateRaw("5", aclList.Head().Id, "0", false, "1"),
			changeCreator.CreateRaw("6", aclList.Head().Id, "0", false, "3", "4", "5"),
		}
		payload = RawChangesPayload{
			NewHeads:   []string{rawChanges[len(rawChanges)-1].Id},
			RawChanges: rawChanges,
		}

		res, err = objTree.AddRawChanges(context.Background(), payload)
		require.NoError(t, err, "adding changes should be without error")

		// check result
		assert.Equal(t, []string{"3"}, res.OldHeads)
		assert.Equal(t, []string{"6"}, res.Heads)
		assert.Equal(t, len(rawChanges), len(res.Added))
		assert.Equal(t, Rebuild, res.Mode)

		// check tree heads
		assert.Equal(t, []string{"6"}, objTree.Heads())

		// check tree iterate
		var iterChangesId []string
		err = objTree.IterateRoot(nil, func(change *Change) bool {
			iterChangesId = append(iterChangesId, change.Id)
			return true
		})
		require.NoError(t, err, "iterate should be without error")
		assert.Equal(t, []string{"0", "1", "2", "3", "4", "5", "6"}, iterChangesId)
		assert.Equal(t, "0", objTree.Root().Id)

		// check storage
		heads, _ := treeStorage.Heads()
		assert.Equal(t, []string{"6"}, heads)

		for _, ch := range rawChanges {
			raw, err := treeStorage.GetRawChange(context.Background(), ch.Id)
			assert.NoError(t, err, "storage should have all the changes")
			assert.Equal(t, ch, raw, "the changes in the storage should be the same")
		}
	})

	t.Run("test history tree not include", func(t *testing.T) {
		changeCreator, deps := prepareTreeDeps(aclList)

		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("1", aclList.Head().Id, "0", false, "0"),
			changeCreator.CreateRaw("2", aclList.Head().Id, "0", false, "1"),
			changeCreator.CreateRaw("3", aclList.Head().Id, "0", true, "2"),
			changeCreator.CreateRaw("4", aclList.Head().Id, "0", false, "2"),
			changeCreator.CreateRaw("5", aclList.Head().Id, "0", false, "1"),
			changeCreator.CreateRaw("6", aclList.Head().Id, "0", false, "3", "4", "5"),
		}
		deps.treeStorage.TransactionAdd(rawChanges, []string{"6"})
		hTree, err := buildHistoryTree(deps, HistoryTreeParams{
			BeforeId:        "6",
			IncludeBeforeId: false,
		})
		require.NoError(t, err)
		// check tree heads
		assert.Equal(t, []string{"3", "4", "5"}, hTree.Heads())

		// check tree iterate
		var iterChangesId []string
		err = hTree.IterateFrom(hTree.Root().Id, nil, func(change *Change) bool {
			iterChangesId = append(iterChangesId, change.Id)
			return true
		})
		require.NoError(t, err, "iterate should be without error")
		assert.Equal(t, []string{"0", "1", "2", "3", "4", "5"}, iterChangesId)
		assert.Equal(t, "0", hTree.Root().Id)
	})

	t.Run("test history tree include", func(t *testing.T) {
		changeCreator, deps := prepareTreeDeps(aclList)

		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("1", aclList.Head().Id, "0", false, "0"),
			changeCreator.CreateRaw("2", aclList.Head().Id, "0", false, "1"),
			changeCreator.CreateRaw("3", aclList.Head().Id, "0", true, "2"),
			changeCreator.CreateRaw("4", aclList.Head().Id, "0", false, "2"),
			changeCreator.CreateRaw("5", aclList.Head().Id, "0", false, "1"),
			changeCreator.CreateRaw("6", aclList.Head().Id, "0", false, "3", "4", "5"),
		}
		deps.treeStorage.TransactionAdd(rawChanges, []string{"6"})
		hTree, err := buildHistoryTree(deps, HistoryTreeParams{
			BeforeId:        "6",
			IncludeBeforeId: true,
		})
		require.NoError(t, err)
		// check tree heads
		assert.Equal(t, []string{"6"}, hTree.Heads())

		// check tree iterate
		var iterChangesId []string
		err = hTree.IterateFrom(hTree.Root().Id, nil, func(change *Change) bool {
			iterChangesId = append(iterChangesId, change.Id)
			return true
		})
		require.NoError(t, err, "iterate should be without error")
		assert.Equal(t, []string{"0", "1", "2", "3", "4", "5", "6"}, iterChangesId)
		assert.Equal(t, "0", hTree.Root().Id)
	})

	t.Run("test history tree root", func(t *testing.T) {
		_, deps := prepareTreeDeps(aclList)
		hTree, err := buildHistoryTree(deps, HistoryTreeParams{
			BeforeId:        "0",
			IncludeBeforeId: true,
		})
		require.NoError(t, err)
		// check tree heads
		assert.Equal(t, []string{"0"}, hTree.Heads())

		// check tree iterate
		var iterChangesId []string
		err = hTree.IterateFrom(hTree.Root().Id, nil, func(change *Change) bool {
			iterChangesId = append(iterChangesId, change.Id)
			return true
		})
		require.NoError(t, err, "iterate should be without error")
		assert.Equal(t, []string{"0"}, iterChangesId)
		assert.Equal(t, "0", hTree.Root().Id)
	})
}

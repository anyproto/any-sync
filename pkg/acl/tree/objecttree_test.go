package tree

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/testutils/acllistbuilder"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

type mockChangeCreator struct{}

func (c *mockChangeCreator) createRaw(id, aclId, snapshotId string, isSnapshot bool, prevIds ...string) *aclpb.RawTreeChangeWithId {
	aclChange := &aclpb.TreeChange{
		TreeHeadIds:    prevIds,
		AclHeadId:      aclId,
		SnapshotBaseId: snapshotId,
		ChangesData:    nil,
		IsSnapshot:     isSnapshot,
	}
	res, _ := aclChange.Marshal()

	raw := &aclpb.RawTreeChange{
		Payload:   res,
		Signature: nil,
	}
	rawMarshalled, _ := raw.Marshal()

	return &aclpb.RawTreeChangeWithId{
		RawChange: rawMarshalled,
		Id:        id,
	}
}

func (c *mockChangeCreator) createNewTreeStorage(treeId, aclListId, aclHeadId, firstChangeId string) storage.TreeStorage {
	firstChange := c.createRaw(firstChangeId, aclHeadId, "", true)
	header := &aclpb.TreeHeader{
		FirstId:        firstChangeId,
		AclId:          aclListId,
		TreeHeaderType: aclpb.TreeHeaderType_Object,
	}
	treeStorage, _ := storage.NewInMemoryTreeStorage(treeId, header, []string{firstChangeId}, []*aclpb.RawTreeChangeWithId{firstChange})
	return treeStorage
}

type mockChangeBuilder struct {
	originalBuilder ChangeBuilder
}

func (c *mockChangeBuilder) ConvertFromRaw(rawChange *aclpb.RawTreeChangeWithId, verify bool) (ch *Change, err error) {
	return c.originalBuilder.ConvertFromRaw(rawChange, false)
}

func (c *mockChangeBuilder) BuildContent(payload BuilderContent) (ch *Change, raw *aclpb.RawTreeChangeWithId, err error) {
	panic("implement me")
}

func (c *mockChangeBuilder) BuildRaw(ch *Change) (raw *aclpb.RawTreeChangeWithId, err error) {
	return c.originalBuilder.BuildRaw(ch)
}

type mockChangeValidator struct{}

func (m *mockChangeValidator) ValidateNewChanges(tree *Tree, aclList list.ACLList, newChanges []*Change) error {
	return nil
}

func (m *mockChangeValidator) ValidateFullTree(tree *Tree, aclList list.ACLList) error {
	return nil
}

type testTreeContext struct {
	aclList       list.ACLList
	treeStorage   storage.TreeStorage
	changeBuilder *mockChangeBuilder
	changeCreator *mockChangeCreator
	objTree       ObjectTree
}

func prepareACLList(t *testing.T) list.ACLList {
	st, err := acllistbuilder.NewListStorageWithTestName("userjoinexample.yml")
	require.NoError(t, err, "building storage should not result in error")

	aclList, err := list.BuildACLList(signingkey.NewEDPubKeyDecoder(), st)
	require.NoError(t, err, "building acl list should be without error")

	return aclList
}

func prepareTreeContext(t *testing.T, aclList list.ACLList) testTreeContext {
	changeCreator := &mockChangeCreator{}
	treeStorage := changeCreator.createNewTreeStorage("treeId", aclList.ID(), aclList.Head().Id, "0")
	changeBuilder := &mockChangeBuilder{
		originalBuilder: newChangeBuilder(nil),
	}
	deps := objectTreeDeps{
		changeBuilder:   changeBuilder,
		treeBuilder:     newTreeBuilder(treeStorage, changeBuilder),
		treeStorage:     treeStorage,
		updateListener:  nil,
		rawChangeLoader: newRawChangeLoader(treeStorage, changeBuilder),
		validator:       &mockChangeValidator{},
		aclList:         aclList,
	}

	// check build
	objTree, err := buildObjectTree(deps)
	require.NoError(t, err, "building tree should be without error")

	// check tree iterate
	var iterChangesId []string
	err = objTree.Iterate(nil, func(change *Change) bool {
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
	aclList := prepareACLList(t)

	t.Run("add simple", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		treeStorage := ctx.treeStorage
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree

		rawChanges := []*aclpb.RawTreeChangeWithId{
			changeCreator.createRaw("1", aclList.Head().Id, "0", false, "0"),
			changeCreator.createRaw("2", aclList.Head().Id, "0", false, "1"),
		}
		res, err := objTree.AddRawChanges(context.Background(), rawChanges...)
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
		err = objTree.Iterate(nil, func(change *Change) bool {
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

		rawChanges := []*aclpb.RawTreeChangeWithId{
			changeCreator.createRaw("0", aclList.Head().Id, "", true, ""),
		}
		res, err := objTree.AddRawChanges(context.Background(), rawChanges...)
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

		rawChanges := []*aclpb.RawTreeChangeWithId{
			changeCreator.createRaw("2", aclList.Head().Id, "0", false, "1"),
		}
		res, err := objTree.AddRawChanges(context.Background(), rawChanges...)
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

		rawChanges := []*aclpb.RawTreeChangeWithId{
			changeCreator.createRaw("1", aclList.Head().Id, "0", false, "0"),
			changeCreator.createRaw("2", aclList.Head().Id, "0", false, "1"),
			changeCreator.createRaw("3", aclList.Head().Id, "0", true, "2"),
			changeCreator.createRaw("4", aclList.Head().Id, "3", false, "3"),
		}
		res, err := objTree.AddRawChanges(context.Background(), rawChanges...)
		require.NoError(t, err, "adding changes should be without error")

		// check result
		assert.Equal(t, []string{"0"}, res.OldHeads)
		assert.Equal(t, []string{"4"}, res.Heads)
		assert.Equal(t, len(rawChanges), len(res.Added))
		assert.Equal(t, Append, res.Mode)

		// check tree heads
		assert.Equal(t, []string{"4"}, objTree.Heads())

		// check tree iterate
		var iterChangesId []string
		err = objTree.Iterate(nil, func(change *Change) bool {
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

		rawChanges := []*aclpb.RawTreeChangeWithId{
			changeCreator.createRaw("1", aclList.Head().Id, "0", false, "0"),
			changeCreator.createRaw("2", aclList.Head().Id, "0", false, "1"),
			changeCreator.createRaw("3", aclList.Head().Id, "0", true, "2"),
		}
		_, err := objTree.AddRawChanges(context.Background(), rawChanges...)
		require.NoError(t, err, "adding changes should be without error")

		snapshotPath := objTree.SnapshotPath()
		assert.Equal(t, []string{"3", "0"}, snapshotPath)

		assert.Equal(t, true, objTree.(*objectTree).snapshotPathIsActual())
	})

	t.Run("changes from tree after common snapshot complex", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree

		rawChanges := []*aclpb.RawTreeChangeWithId{
			changeCreator.createRaw("1", aclList.Head().Id, "0", false, "0"),
			changeCreator.createRaw("2", aclList.Head().Id, "0", false, "1"),
			changeCreator.createRaw("3", aclList.Head().Id, "0", true, "2"),
			changeCreator.createRaw("4", aclList.Head().Id, "0", false, "2"),
			changeCreator.createRaw("5", aclList.Head().Id, "0", false, "1"),
			changeCreator.createRaw("6", aclList.Head().Id, "0", false, "3", "4", "5"),
		}

		_, err := objTree.AddRawChanges(context.Background(), rawChanges...)
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

		rawChanges := []*aclpb.RawTreeChangeWithId{
			changeCreator.createRaw("1", aclList.Head().Id, "0", false, "0"),
			changeCreator.createRaw("2", aclList.Head().Id, "0", false, "1"),
			changeCreator.createRaw("3", aclList.Head().Id, "0", true, "2"),
			changeCreator.createRaw("4", aclList.Head().Id, "0", false, "2"),
			changeCreator.createRaw("5", aclList.Head().Id, "0", false, "1"),
			// main difference from tree example
			changeCreator.createRaw("6", aclList.Head().Id, "0", true, "3", "4", "5"),
		}

		_, err := objTree.AddRawChanges(context.Background(), rawChanges...)
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

		rawChanges := []*aclpb.RawTreeChangeWithId{
			changeCreator.createRaw("1", aclList.Head().Id, "0", false, "0"),
			changeCreator.createRaw("2", aclList.Head().Id, "0", false, "1"),
			changeCreator.createRaw("3", aclList.Head().Id, "0", true, "2"),
		}
		res, err := objTree.AddRawChanges(context.Background(), rawChanges...)
		require.NoError(t, err, "adding changes should be without error")
		require.Equal(t, "3", objTree.Root().Id)

		rawChanges = []*aclpb.RawTreeChangeWithId{
			changeCreator.createRaw("4", aclList.Head().Id, "0", false, "2"),
			changeCreator.createRaw("5", aclList.Head().Id, "0", false, "1"),
			changeCreator.createRaw("6", aclList.Head().Id, "0", false, "3", "4", "5"),
		}
		res, err = objTree.AddRawChanges(context.Background(), rawChanges...)
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
		err = objTree.Iterate(nil, func(change *Change) bool {
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
}

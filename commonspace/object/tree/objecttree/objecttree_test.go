package objecttree

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/object/accountdata"
	"github.com/anytypeio/any-sync/commonspace/object/acl/list"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type testTreeContext struct {
	aclList       list.AclList
	treeStorage   treestorage.TreeStorage
	changeCreator *MockChangeCreator
	objTree       ObjectTree
}

func prepareAclList(t *testing.T) (list.AclList, *accountdata.AccountKeys) {
	randKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	aclList, err := list.NewTestDerivedAcl("spaceId", randKeys)
	require.NoError(t, err, "building acl list should be without error")

	return aclList, randKeys
}

func prepareHistoryTreeDeps(aclList list.AclList) (*MockChangeCreator, objectTreeDeps) {
	changeCreator := NewMockChangeCreator()
	treeStorage := changeCreator.CreateNewTreeStorage("0", aclList.Head().Id)
	root, _ := treeStorage.Root()
	changeBuilder := &nonVerifiableChangeBuilder{
		ChangeBuilder: NewChangeBuilder(newMockKeyStorage(), root),
	}
	deps := objectTreeDeps{
		changeBuilder:   changeBuilder,
		treeBuilder:     newTreeBuilder(true, treeStorage, changeBuilder),
		treeStorage:     treeStorage,
		rawChangeLoader: newRawChangeLoader(treeStorage, changeBuilder),
		validator:       &noOpTreeValidator{},
		aclList:         aclList,
	}
	return changeCreator, deps
}

func prepareTreeContext(t *testing.T, aclList list.AclList) testTreeContext {
	return prepareContext(t, aclList, BuildTestableTree, nil)
}

func prepareEmptyDataTreeContext(t *testing.T, aclList list.AclList, additionalChanges func(changeCreator *MockChangeCreator) RawChangesPayload) testTreeContext {
	return prepareContext(t, aclList, BuildEmptyDataTestableTree, additionalChanges)
}

func prepareContext(
	t *testing.T,
	aclList list.AclList,
	objTreeBuilder BuildObjectTreeFunc,
	additionalChanges func(changeCreator *MockChangeCreator) RawChangesPayload) testTreeContext {
	changeCreator := NewMockChangeCreator()
	treeStorage := changeCreator.CreateNewTreeStorage("0", aclList.Head().Id)
	if additionalChanges != nil {
		payload := additionalChanges(changeCreator)
		err := treeStorage.TransactionAdd(payload.RawChanges, payload.NewHeads)
		require.NoError(t, err)
	}
	objTree, err := objTreeBuilder(treeStorage, aclList)
	require.NoError(t, err, "building tree should be without error")

	// check tree iterate
	var iterChangesId []string
	err = objTree.IterateRoot(nil, func(change *Change) bool {
		iterChangesId = append(iterChangesId, change.Id)
		return true
	})
	require.NoError(t, err, "iterate should be without error")
	if additionalChanges == nil {
		assert.Equal(t, []string{"0"}, iterChangesId)
	}
	return testTreeContext{
		aclList:       aclList,
		treeStorage:   treeStorage,
		changeCreator: changeCreator,
		objTree:       objTree,
	}
}

func TestObjectTree(t *testing.T) {
	aclList, keys := prepareAclList(t)
	ctx := context.Background()

	t.Run("add content", func(t *testing.T) {
		root, err := CreateObjectTreeRoot(ObjectTreeCreatePayload{
			PrivKey:       keys.SignKey,
			ChangeType:    "changeType",
			ChangePayload: nil,
			SpaceId:       "spaceId",
			IsEncrypted:   true,
		}, aclList)
		require.NoError(t, err)
		store, _ := treestorage.NewInMemoryTreeStorage(root, []string{root.Id}, []*treechangeproto.RawTreeChangeWithId{root})
		oTree, err := BuildObjectTree(store, aclList)
		require.NoError(t, err)

		t.Run("0 timestamp is changed", func(t *testing.T) {
			res, err := oTree.AddContent(ctx, SignableChangeContent{
				Data:        []byte("some"),
				Key:         keys.SignKey,
				IsSnapshot:  false,
				IsEncrypted: true,
				Timestamp:   0,
			})
			require.NoError(t, err)
			require.Len(t, oTree.Heads(), 1)
			require.Equal(t, res.Added[0].Id, oTree.Heads()[0])
			ch, err := oTree.(*objectTree).changeBuilder.Unmarshall(res.Added[0], true)
			require.NoError(t, err)
			require.NotZero(t, ch.Timestamp)
		})
		t.Run("timestamp is set correctly", func(t *testing.T) {
			someTs := time.Now().Add(time.Hour).Unix()
			res, err := oTree.AddContent(ctx, SignableChangeContent{
				Data:        []byte("some"),
				Key:         keys.SignKey,
				IsSnapshot:  false,
				IsEncrypted: true,
				Timestamp:   someTs,
			})
			require.NoError(t, err)
			require.Len(t, oTree.Heads(), 1)
			require.Equal(t, res.Added[0].Id, oTree.Heads()[0])
			ch, err := oTree.(*objectTree).changeBuilder.Unmarshall(res.Added[0], true)
			require.NoError(t, err)
			require.Equal(t, ch.Timestamp, someTs)
		})
	})

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

	t.Run("add not connected changes", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree

		// this change could in theory replace current snapshot, we should prevent that
		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("2", aclList.Head().Id, "0", true, "1"),
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

	t.Run("test empty data tree", func(t *testing.T) {
		t.Run("empty tree add", func(t *testing.T) {
			ctx := prepareEmptyDataTreeContext(t, aclList, nil)
			changeCreator := ctx.changeCreator
			objTree := ctx.objTree

			rawChangesFirst := []*treechangeproto.RawTreeChangeWithId{
				changeCreator.CreateRawWithData("1", aclList.Head().Id, "0", false, []byte("1"), "0"),
				changeCreator.CreateRawWithData("2", aclList.Head().Id, "0", false, []byte("2"), "1"),
				changeCreator.CreateRawWithData("3", aclList.Head().Id, "0", false, []byte("3"), "2"),
			}
			rawChangesSecond := []*treechangeproto.RawTreeChangeWithId{
				changeCreator.CreateRawWithData("4", aclList.Head().Id, "0", false, []byte("4"), "2"),
				changeCreator.CreateRawWithData("5", aclList.Head().Id, "0", false, []byte("5"), "1"),
				changeCreator.CreateRawWithData("6", aclList.Head().Id, "0", false, []byte("6"), "3", "4", "5"),
			}

			// making them to be saved in unattached
			_, err := objTree.AddRawChanges(context.Background(), RawChangesPayload{
				NewHeads:   []string{"6"},
				RawChanges: rawChangesSecond,
			})
			require.NoError(t, err, "adding changes should be without error")
			// attaching them
			res, err := objTree.AddRawChanges(context.Background(), RawChangesPayload{
				NewHeads:   []string{"3"},
				RawChanges: rawChangesFirst,
			})

			require.NoError(t, err, "adding changes should be without error")
			require.Equal(t, "0", objTree.Root().Id)
			require.Equal(t, []string{"6"}, objTree.Heads())
			require.Equal(t, 6, len(res.Added))

			// checking that added changes still have data
			for _, ch := range res.Added {
				unmarshallRaw := &treechangeproto.RawTreeChange{}
				proto.Unmarshal(ch.RawChange, unmarshallRaw)
				treeCh := &treechangeproto.TreeChange{}
				proto.Unmarshal(unmarshallRaw.Payload, treeCh)
				require.Equal(t, ch.Id, string(treeCh.ChangesData))
			}

			// checking that the tree doesn't have data in memory
			err = objTree.IterateRoot(nil, func(change *Change) bool {
				if change.Id == "0" {
					return true
				}
				require.Nil(t, change.Data)
				return true
			})
		})

		t.Run("empty tree load", func(t *testing.T) {
			ctx := prepareEmptyDataTreeContext(t, aclList, func(changeCreator *MockChangeCreator) RawChangesPayload {
				rawChanges := []*treechangeproto.RawTreeChangeWithId{
					changeCreator.CreateRawWithData("1", aclList.Head().Id, "0", false, []byte("1"), "0"),
					changeCreator.CreateRawWithData("2", aclList.Head().Id, "0", false, []byte("2"), "1"),
					changeCreator.CreateRawWithData("3", aclList.Head().Id, "0", false, []byte("3"), "2"),
					changeCreator.CreateRawWithData("4", aclList.Head().Id, "0", false, []byte("4"), "2"),
					changeCreator.CreateRawWithData("5", aclList.Head().Id, "0", false, []byte("5"), "1"),
					changeCreator.CreateRawWithData("6", aclList.Head().Id, "0", false, []byte("6"), "3", "4", "5"),
				}
				return RawChangesPayload{NewHeads: []string{"6"}, RawChanges: rawChanges}
			})
			ctx.objTree.IterateRoot(nil, func(change *Change) bool {
				if change.Id == "0" {
					return true
				}
				require.Nil(t, change.Data)
				return true
			})
			rawChanges, err := ctx.objTree.ChangesAfterCommonSnapshot([]string{"0"}, []string{"6"})
			require.NoError(t, err)
			for _, ch := range rawChanges {
				unmarshallRaw := &treechangeproto.RawTreeChange{}
				proto.Unmarshal(ch.RawChange, unmarshallRaw)
				treeCh := &treechangeproto.TreeChange{}
				proto.Unmarshal(unmarshallRaw.Payload, treeCh)
				require.Equal(t, ch.Id, string(treeCh.ChangesData))
			}
		})
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
		changeCreator, deps := prepareHistoryTreeDeps(aclList)

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
		changeCreator, deps := prepareHistoryTreeDeps(aclList)

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
		_, deps := prepareHistoryTreeDeps(aclList)
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

	t.Run("validate correct tree", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		changeCreator := ctx.changeCreator

		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			ctx.objTree.Header(),
			changeCreator.CreateRaw("1", aclList.Head().Id, "0", false, "0"),
			changeCreator.CreateRaw("2", aclList.Head().Id, "0", false, "1"),
			changeCreator.CreateRaw("3", aclList.Head().Id, "0", true, "2"),
		}
		defaultObjectTreeDeps = nonVerifiableTreeDeps
		err := ValidateRawTree(treestorage.TreeStorageCreatePayload{
			RootRawChange: ctx.objTree.Header(),
			Heads:         []string{"3"},
			Changes:       rawChanges,
		}, ctx.aclList)
		require.NoError(t, err)
	})

	t.Run("fail to validate not connected tree", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		changeCreator := ctx.changeCreator

		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			ctx.objTree.Header(),
			changeCreator.CreateRaw("3", aclList.Head().Id, "0", true, "2"),
		}
		defaultObjectTreeDeps = nonVerifiableTreeDeps
		err := ValidateRawTree(treestorage.TreeStorageCreatePayload{
			RootRawChange: ctx.objTree.Header(),
			Heads:         []string{"3"},
			Changes:       rawChanges,
		}, ctx.aclList)
		require.Equal(t, ErrHasInvalidChanges, err)
	})
}

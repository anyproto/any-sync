package objecttree

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"golang.org/x/exp/slices"

	"github.com/anyproto/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/liststorage"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
)

// genParams is the parameters for genChanges
type genParams struct {
	// prefix is the prefix which is added to change id
	prefix     string
	aclId      string
	startIdx   int
	levels     int
	perLevel   int
	snapshotId string
	prevHeads  []string
	isSnapshot func() bool
	hasData    bool
}

// genResult is the result of genChanges
type genResult struct {
	changes    []*treechangeproto.RawTreeChangeWithId
	heads      []string
	snapshotId string
}

// genChanges generates several levels of tree changes where each level is connected only with previous one
func genChanges(creator *MockChangeCreator, params genParams) (res genResult) {
	src := rand.NewSource(time.Now().Unix())
	rnd := rand.New(src)
	var (
		prevHeads  []string
		snapshotId = params.snapshotId
	)
	prevHeads = append(prevHeads, params.prevHeads...)

	for i := 0; i < params.levels; i++ {
		var (
			newHeads []string
			usedIds  = map[string]struct{}{}
		)
		newChange := func(isSnapshot bool, idx int, prevIds []string) string {
			newId := fmt.Sprintf("%s.%d.%d", params.prefix, params.startIdx+i, idx)
			var data []byte
			if params.hasData {
				data = []byte(newId)
			}
			newCh := creator.CreateRawWithData(newId, params.aclId, snapshotId, isSnapshot, data, prevIds...)
			res.changes = append(res.changes, newCh)
			return newId
		}
		if params.isSnapshot() {
			newId := newChange(true, 0, prevHeads)
			prevHeads = []string{newId}
			snapshotId = newId
			continue
		}
		perLevel := rnd.Intn(params.perLevel)
		if perLevel == 0 {
			perLevel = 1
		}
		for j := 0; j < perLevel; j++ {
			prevConns := rnd.Intn(len(prevHeads))
			if prevConns == 0 {
				prevConns = 1
			}
			rnd.Shuffle(len(prevHeads), func(i, j int) {
				prevHeads[i], prevHeads[j] = prevHeads[j], prevHeads[i]
			})
			// if we didn't connect with all prev ones
			if j == perLevel-1 && len(usedIds) != len(prevHeads) {
				var unusedIds []string
				for _, id := range prevHeads {
					if _, exists := usedIds[id]; !exists {
						unusedIds = append(unusedIds, id)
					}
				}
				prevHeads = unusedIds
				prevConns = len(prevHeads)
			}
			var prevIds []string
			for k := 0; k < prevConns; k++ {
				prevIds = append(prevIds, prevHeads[k])
				usedIds[prevHeads[k]] = struct{}{}
			}
			newId := newChange(false, j, prevIds)
			newHeads = append(newHeads, newId)
		}
		prevHeads = newHeads
	}
	res.heads = prevHeads
	res.snapshotId = snapshotId
	return
}

type testTreeContext struct {
	aclList       list.AclList
	treeStorage   treestorage.TreeStorage
	changeCreator *MockChangeCreator
	objTree       ObjectTree
}

func genBuildFilterableTestableTree(filterFunc func(ch *Change) bool) func(treeStorage treestorage.TreeStorage, aclList list.AclList) (ObjectTree, error) {
	return func(treeStorage treestorage.TreeStorage, aclList list.AclList) (ObjectTree, error) {
		root, _ := treeStorage.Root()
		changeBuilder := &nonVerifiableChangeBuilder{
			ChangeBuilder: NewChangeBuilder(newMockKeyStorage(), root),
		}
		loader := newRawChangeLoader(treeStorage, changeBuilder)
		deps := objectTreeDeps{
			changeBuilder:   changeBuilder,
			treeBuilder:     newTreeBuilder(true, treeStorage, changeBuilder, loader),
			treeStorage:     treeStorage,
			rawChangeLoader: loader,
			validator:       &noOpTreeValidator{filterFunc: filterFunc},
			aclList:         aclList,
			flusher:         &defaultFlusher{},
		}

		return buildObjectTree(deps)
	}
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
	treeStorage := changeCreator.CreateNewTreeStorage("0", aclList.Head().Id, false)
	root, _ := treeStorage.Root()
	changeBuilder := &nonVerifiableChangeBuilder{
		ChangeBuilder: NewChangeBuilder(newMockKeyStorage(), root),
	}
	loader := newRawChangeLoader(treeStorage, changeBuilder)
	deps := objectTreeDeps{
		changeBuilder:   changeBuilder,
		treeBuilder:     newTreeBuilder(true, treeStorage, changeBuilder, loader),
		treeStorage:     treeStorage,
		rawChangeLoader: loader,
		validator:       &noOpTreeValidator{},
		aclList:         aclList,
	}
	return changeCreator, deps
}

func prepareTreeContext(t *testing.T, aclList list.AclList) testTreeContext {
	return prepareContext(t, aclList, BuildTestableTree, false, nil)
}

func prepareDerivedTreeContext(t *testing.T, aclList list.AclList) testTreeContext {
	return prepareContext(t, aclList, BuildTestableTree, true, nil)
}

func prepareEmptyDataTreeContext(t *testing.T, aclList list.AclList, additionalChanges func(changeCreator *MockChangeCreator) RawChangesPayload) testTreeContext {
	return prepareContext(t, aclList, BuildEmptyDataTestableTree, false, additionalChanges)
}

func prepareContext(
	t *testing.T,
	aclList list.AclList,
	objTreeBuilder BuildObjectTreeFunc,
	isDerived bool,
	additionalChanges func(changeCreator *MockChangeCreator) RawChangesPayload) testTreeContext {
	changeCreator := NewMockChangeCreator()
	treeStorage := changeCreator.CreateNewTreeStorage("0", aclList.Head().Id, isDerived)
	if additionalChanges != nil {
		payload := additionalChanges(changeCreator)
		err := treeStorage.AddRawChangesSetHeads(payload.RawChanges, payload.NewHeads)
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

	t.Run("delete object tree", func(t *testing.T) {
		exec := list.NewAclExecutor("spaceId")
		type cmdErr struct {
			cmd string
			err error
		}
		cmds := []cmdErr{
			{"a.init::a", nil},
		}
		for _, cmd := range cmds {
			err := exec.Execute(cmd.cmd)
			require.Equal(t, cmd.err, err, cmd)
		}
		aAccount := exec.ActualAccounts()["a"]
		root, err := CreateObjectTreeRoot(ObjectTreeCreatePayload{
			PrivKey:       aAccount.Keys.SignKey,
			ChangeType:    "changeType",
			ChangePayload: nil,
			SpaceId:       "spaceId",
			IsEncrypted:   true,
		}, aAccount.Acl)
		require.NoError(t, err)
		aStore, _ := treestorage.NewInMemoryTreeStorage(root, []string{root.Id}, []*treechangeproto.RawTreeChangeWithId{root})
		aTree, err := BuildKeyFilterableObjectTree(aStore, aAccount.Acl)
		require.NoError(t, err)
		err = aTree.Delete()
		require.NoError(t, err)
		_, err = aTree.ChangesAfterCommonSnapshot(nil, nil)
		require.Equal(t, ErrDeleted, err)
		_, err = aTree.ChangesAfterCommonSnapshotLoader(nil, nil)
		require.Equal(t, ErrDeleted, err)
		err = aTree.IterateFrom("", nil, func(change *Change) bool {
			return true
		})
		require.Equal(t, ErrDeleted, err)
		_, err = aTree.AddContent(ctx, SignableChangeContent{})
		require.Equal(t, ErrDeleted, err)
		_, err = aTree.AddRawChanges(ctx, RawChangesPayload{})
		require.Equal(t, ErrDeleted, err)
		_, err = aTree.PrepareChange(SignableChangeContent{})
		require.Equal(t, ErrDeleted, err)
		_, err = aTree.UnpackChange(nil)
		require.Equal(t, ErrDeleted, err)
		_, err = aTree.GetChange("")
		require.Equal(t, ErrDeleted, err)
	})

	t.Run("user delete logic: validation, key change, decryption", func(t *testing.T) {
		exec := list.NewAclExecutor("spaceId")
		type cmdErr struct {
			cmd string
			err error
		}
		cmds := []cmdErr{
			{"a.init::a", nil},
			{"a.invite::invId", nil},
			{"b.join::invId", nil},
			{"a.approve::b,r", nil},
		}
		for _, cmd := range cmds {
			err := exec.Execute(cmd.cmd)
			require.Equal(t, cmd.err, err, cmd)
		}
		aAccount := exec.ActualAccounts()["a"]
		bAccount := exec.ActualAccounts()["b"]
		root, err := CreateObjectTreeRoot(ObjectTreeCreatePayload{
			PrivKey:       aAccount.Keys.SignKey,
			ChangeType:    "changeType",
			ChangePayload: nil,
			SpaceId:       "spaceId",
			IsEncrypted:   true,
		}, aAccount.Acl)
		require.NoError(t, err)
		aStore, _ := treestorage.NewInMemoryTreeStorage(root, []string{root.Id}, []*treechangeproto.RawTreeChangeWithId{root})
		aTree, err := BuildKeyFilterableObjectTree(aStore, aAccount.Acl)
		require.NoError(t, err)
		_, err = aTree.AddContent(ctx, SignableChangeContent{
			Data:        []byte("some"),
			Key:         aAccount.Keys.SignKey,
			IsSnapshot:  false,
			IsEncrypted: true,
			DataType:    mockDataType,
		})
		require.NoError(t, err)
		bStore := aTree.Storage().(*treestorage.InMemoryTreeStorage).Copy()
		bTree, err := BuildKeyFilterableObjectTree(bStore, bAccount.Acl)
		require.NoError(t, err)
		err = exec.Execute("a.remove::b")
		require.NoError(t, err)
		res, err := aTree.AddContent(ctx, SignableChangeContent{
			Data:        []byte("some"),
			Key:         aAccount.Keys.SignKey,
			IsSnapshot:  false,
			IsEncrypted: true,
			DataType:    mockDataType,
		})
		require.NoError(t, err)
		oldHeads := bTree.Heads()
		res, err = bTree.AddRawChanges(ctx, RawChangesPayload{
			NewHeads:   aTree.Heads(),
			RawChanges: res.Added,
		})
		require.Equal(t, oldHeads, bTree.Heads())
		bStore = aTree.Storage().(*treestorage.InMemoryTreeStorage).Copy()
		root, _ = bStore.Root()
		heads, _ := bStore.Heads()
		newTree, err := ValidateFilterRawTree(treestorage.TreeStorageCreatePayload{
			RootRawChange: root,
			Changes:       bStore.AllChanges(),
			Heads:         heads,
		}, InMemoryStorageCreator{}, bAccount.Acl)
		require.NoError(t, err)
		require.Equal(t, 2, len(newTree.Storage().(*treestorage.InMemoryTreeStorage).AllChanges()))
		err = aTree.IterateRoot(func(change *Change, decrypted []byte) (any, error) {
			return nil, nil
		}, func(change *Change) bool {
			return true
		})
		require.NoError(t, err)
	})

	t.Run("reject root referring to unknown acl", func(t *testing.T) {
		exec := list.NewAclExecutor("spaceId")
		type cmdErr struct {
			cmd string
			err error
		}
		cmds := []cmdErr{
			{"a.init::a", nil},
		}
		for _, cmd := range cmds {
			err := exec.Execute(cmd.cmd)
			require.Equal(t, cmd.err, err, cmd)
		}
		account := exec.ActualAccounts()["a"]
		recs, err := account.Acl.RecordsAfter(ctx, "")
		require.NoError(t, err)
		beforeStorage, err := liststorage.NewInMemoryAclListStorage(recs[0].Id, recs)
		require.NoError(t, err)
		beforeAcl, err := list.BuildAclListWithIdentity(account.Keys, beforeStorage, list.NoOpAcceptorVerifier{})
		require.NoError(t, err)
		err = exec.Execute("a.invite::invId")
		require.NoError(t, err)
		root, err := CreateObjectTreeRoot(ObjectTreeCreatePayload{
			PrivKey:       account.Keys.SignKey,
			ChangeType:    "changeType",
			ChangePayload: nil,
			SpaceId:       "spaceId",
			IsEncrypted:   true,
		}, account.Acl)
		require.NoError(t, err)
		treeStorage, err := treestorage.NewInMemoryTreeStorage(root, []string{root.Id}, []*treechangeproto.RawTreeChangeWithId{root})
		require.NoError(t, err)
		_, err = BuildKeyFilterableObjectTree(treeStorage, beforeAcl)
		require.Equal(t, list.ErrNoSuchRecord, err)
	})

	t.Run("filter changes when no aclHeadId", func(t *testing.T) {
		exec := list.NewAclExecutor("spaceId")
		type cmdErr struct {
			cmd string
			err error
		}
		cmds := []cmdErr{
			{"a.init::a", nil},
			{"a.invite::invId", nil},
			{"b.join::invId", nil},
			{"a.approve::b,r", nil},
		}
		for _, cmd := range cmds {
			err := exec.Execute(cmd.cmd)
			require.Equal(t, cmd.err, err, cmd)
		}
		aAccount := exec.ActualAccounts()["a"]
		bAccount := exec.ActualAccounts()["b"]
		root, err := CreateObjectTreeRoot(ObjectTreeCreatePayload{
			PrivKey:       aAccount.Keys.SignKey,
			ChangeType:    "changeType",
			ChangePayload: nil,
			SpaceId:       "spaceId",
			IsEncrypted:   true,
		}, aAccount.Acl)
		require.NoError(t, err)
		aStore, _ := treestorage.NewInMemoryTreeStorage(root, []string{root.Id}, []*treechangeproto.RawTreeChangeWithId{root})
		aTree, err := BuildKeyFilterableObjectTree(aStore, aAccount.Acl)
		require.NoError(t, err)
		_, err = aTree.AddContent(ctx, SignableChangeContent{
			Data:        []byte("some"),
			Key:         aAccount.Keys.SignKey,
			IsSnapshot:  false,
			IsEncrypted: true,
			DataType:    mockDataType,
		})
		require.NoError(t, err)
		bStore := aTree.Storage().(*treestorage.InMemoryTreeStorage).Copy()
		// copying old version of storage
		prevAclRecs, err := bAccount.Acl.RecordsAfter(ctx, "")
		require.NoError(t, err)
		storage, err := liststorage.NewInMemoryAclListStorage(prevAclRecs[0].Id, prevAclRecs)
		require.NoError(t, err)
		acl, err := list.BuildAclListWithIdentity(bAccount.Keys, storage, list.NoOpAcceptorVerifier{})
		require.NoError(t, err)
		// creating tree with old storage which doesn't have a new invite record
		bTree, err := BuildKeyFilterableObjectTree(bStore, acl)
		require.NoError(t, err)
		err = exec.Execute("a.invite::inv1Id")
		require.NoError(t, err)
		res, err := aTree.AddContent(ctx, SignableChangeContent{
			Data:        []byte("some"),
			Key:         aAccount.Keys.SignKey,
			IsSnapshot:  false,
			IsEncrypted: true,
			DataType:    mockDataType,
		})
		unexpectedId := res.Added[0].Id
		require.NoError(t, err)
		var collectedChanges []*Change
		err = aTree.IterateRoot(func(change *Change, decrypted []byte) (any, error) {
			return nil, nil
		}, func(change *Change) bool {
			collectedChanges = append(collectedChanges, change)
			return true
		})
		require.NoError(t, err)
		bObjTree := bTree.(*objectTree)
		// this is just a random slice, so the func works
		indexes := []int{1, 2, 3, 4, 5}
		// checking that we filter the changes
		filtered, filteredChanges, _, _ := bObjTree.validator.FilterChanges(bObjTree.aclList, collectedChanges, nil, indexes)
		require.True(t, filtered)
		for _, ch := range filteredChanges {
			require.NotEqual(t, unexpectedId, ch.Id)
		}
	})

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

		t.Run("add content validate failed", func(t *testing.T) {
			_, err := oTree.AddContentWithValidator(ctx, SignableChangeContent{
				Data:        []byte("some"),
				Key:         keys.SignKey,
				IsSnapshot:  false,
				IsEncrypted: true,
				Timestamp:   0,
				DataType:    mockDataType,
			}, func(change *treechangeproto.RawTreeChangeWithId) error {
				return errors.New("validation failed")
			})
			require.Error(t, err)
			require.Len(t, oTree.Heads(), 1)
			require.Equal(t, root.Id, oTree.Root().Id)
		})
		t.Run("0 timestamp is changed to current, data type is correct", func(t *testing.T) {
			start := time.Now()
			res, err := oTree.AddContent(ctx, SignableChangeContent{
				Data:        []byte("some"),
				Key:         keys.SignKey,
				IsSnapshot:  false,
				IsEncrypted: true,
				Timestamp:   0,
				DataType:    mockDataType,
			})
			end := time.Now()
			require.NoError(t, err)
			require.Len(t, oTree.Heads(), 1)
			require.Equal(t, res.Added[0].Id, oTree.Heads()[0])
			ch, err := oTree.(*objectTree).changeBuilder.Unmarshall(res.Added[0], true)
			require.NoError(t, err)
			require.GreaterOrEqual(t, start.Unix(), ch.Timestamp)
			require.LessOrEqual(t, end.Unix(), ch.Timestamp)
			require.Equal(t, res.Added[0].Id, oTree.(*objectTree).tree.lastIteratedHeadId)
			require.Equal(t, mockDataType, ch.DataType)
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
			require.Equal(t, res.Added[0].Id, oTree.(*objectTree).tree.lastIteratedHeadId)
		})
	})

	t.Run("validate", func(t *testing.T) {
		t.Run("non-derived only root is ok", func(t *testing.T) {
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
			emptyDataTreeDeps = nonVerifiableTreeDeps
			err = ValidateRawTree(treestorage.TreeStorageCreatePayload{
				RootRawChange: oTree.Header(),
				Heads:         []string{root.Id},
				Changes:       oTree.Storage().(*treestorage.InMemoryTreeStorage).AllChanges(),
			}, aclList)
			require.NoError(t, err)
		})

		t.Run("derived only root incorrect", func(t *testing.T) {
			root, err := DeriveObjectTreeRoot(ObjectTreeDerivePayload{
				ChangeType:    "changeType",
				ChangePayload: nil,
				SpaceId:       "spaceId",
				IsEncrypted:   true,
			}, aclList)
			require.NoError(t, err)
			store, _ := treestorage.NewInMemoryTreeStorage(root, []string{root.Id}, []*treechangeproto.RawTreeChangeWithId{root})
			oTree, err := BuildObjectTree(store, aclList)
			require.NoError(t, err)
			emptyDataTreeDeps = nonVerifiableTreeDeps
			err = ValidateRawTree(treestorage.TreeStorageCreatePayload{
				RootRawChange: oTree.Header(),
				Heads:         []string{root.Id},
				Changes:       oTree.Storage().(*treestorage.InMemoryTreeStorage).AllChanges(),
			}, aclList)
			require.Equal(t, ErrDerived, err)
		})

		t.Run("derived root returns true", func(t *testing.T) {
			root, err := DeriveObjectTreeRoot(ObjectTreeDerivePayload{
				ChangeType:    "changeType",
				ChangePayload: nil,
				SpaceId:       "spaceId",
				IsEncrypted:   true,
			}, aclList)
			require.NoError(t, err)
			isDerived, err := IsDerivedRoot(root)
			require.NoError(t, err)
			require.True(t, isDerived)
		})

		t.Run("derived more than 1 change, not snapshot, correct", func(t *testing.T) {
			root, err := DeriveObjectTreeRoot(ObjectTreeDerivePayload{
				ChangeType:    "changeType",
				ChangePayload: nil,
				SpaceId:       "spaceId",
				IsEncrypted:   true,
			}, aclList)
			require.NoError(t, err)
			store, _ := treestorage.NewInMemoryTreeStorage(root, []string{root.Id}, []*treechangeproto.RawTreeChangeWithId{root})
			oTree, err := BuildObjectTree(store, aclList)
			require.NoError(t, err)
			_, err = oTree.AddContent(ctx, SignableChangeContent{
				Data:        []byte("some"),
				Key:         keys.SignKey,
				IsSnapshot:  false,
				IsEncrypted: true,
				Timestamp:   time.Now().Unix(),
				DataType:    mockDataType,
			})
			require.NoError(t, err)
			allChanges := oTree.Storage().(*treestorage.InMemoryTreeStorage).AllChanges()
			emptyDataTreeDeps = nonVerifiableTreeDeps
			err = ValidateRawTree(treestorage.TreeStorageCreatePayload{
				RootRawChange: oTree.Header(),
				Heads:         []string{oTree.Heads()[0]},
				Changes:       allChanges,
			}, aclList)
			require.NoError(t, err)
			rawChs, err := oTree.ChangesAfterCommonSnapshot(nil, nil)
			require.NoError(t, err)
			sortFunc := func(a, b *treechangeproto.RawTreeChangeWithId) int {
				if a.Id < b.Id {
					return 1
				} else {
					return -1
				}
			}
			slices.SortFunc(allChanges, sortFunc)
			slices.SortFunc(rawChs, sortFunc)
			require.Equal(t, allChanges, rawChs)
		})

		t.Run("derived more than 1 change, snapshot, correct", func(t *testing.T) {
			root, err := DeriveObjectTreeRoot(ObjectTreeDerivePayload{
				ChangeType:    "changeType",
				ChangePayload: nil,
				SpaceId:       "spaceId",
				IsEncrypted:   true,
			}, aclList)
			require.NoError(t, err)
			store, _ := treestorage.NewInMemoryTreeStorage(root, []string{root.Id}, []*treechangeproto.RawTreeChangeWithId{root})
			emptyDataTreeDeps = nonVerifiableTreeDeps
			oTree, err := BuildObjectTree(store, aclList)
			require.NoError(t, err)
			_, err = oTree.AddContent(ctx, SignableChangeContent{
				Data:        []byte("some"),
				Key:         keys.SignKey,
				IsSnapshot:  true,
				IsEncrypted: true,
				Timestamp:   time.Now().Unix(),
				DataType:    mockDataType,
			})
			require.NoError(t, err)
			allChanges := oTree.Storage().(*treestorage.InMemoryTreeStorage).AllChanges()
			err = ValidateRawTree(treestorage.TreeStorageCreatePayload{
				RootRawChange: oTree.Header(),
				Heads:         []string{oTree.Heads()[0]},
				Changes:       allChanges,
			}, aclList)
			require.NoError(t, err)
			rawChs, err := oTree.ChangesAfterCommonSnapshot(nil, nil)
			require.NoError(t, err)
			sortFunc := func(a, b *treechangeproto.RawTreeChangeWithId) int {
				if a.Id < b.Id {
					return 1
				} else {
					return -1
				}
			}
			slices.SortFunc(allChanges, sortFunc)
			slices.SortFunc(rawChs, sortFunc)
			require.Equal(t, allChanges, rawChs)
		})

		t.Run("validate from start, multiple snapshots, correct", func(t *testing.T) {
			ctx := prepareTreeContext(t, aclList)
			changeCreator := ctx.changeCreator

			rawChanges := []*treechangeproto.RawTreeChangeWithId{
				ctx.objTree.Header(),
				changeCreator.CreateRaw("1", aclList.Head().Id, "0", false, "0"),
				changeCreator.CreateRaw("2", aclList.Head().Id, "0", false, "1"),
				changeCreator.CreateRaw("3", aclList.Head().Id, "0", true, "2"),
			}
			emptyDataTreeDeps = nonVerifiableTreeDeps
			err := ValidateRawTree(treestorage.TreeStorageCreatePayload{
				RootRawChange: ctx.objTree.Header(),
				Heads:         []string{"3"},
				Changes:       rawChanges,
			}, ctx.aclList)
			require.NoError(t, err)
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
			if change.Id != objTree.Id() {
				assert.Equal(t, mockDataType, change.DataType)
			}
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

	t.Run("add with rollback", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree

		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("1", aclList.Head().Id, "0", false, "0"),
			changeCreator.CreateRaw("2", aclList.Head().Id, "0", false, "1"),
			changeCreator.CreateRaw("3", aclList.Head().Id, "0", false, "2"),
			changeCreator.CreateRaw("4", aclList.Head().Id, "0", false, "3"),
		}
		payload := RawChangesPayload{
			NewHeads:   []string{rawChanges[len(rawChanges)-1].Id},
			RawChanges: rawChanges,
		}
		tr := objTree.(*objectTree)
		tr.validator.(*noOpTreeValidator).fail = true
		_, err := objTree.AddRawChanges(context.Background(), payload)
		require.Error(t, err)
		require.Len(t, tr.tree.attached, 1)
		require.Empty(t, tr.tree.attached["0"].Next)
	})

	t.Run("add new snapshot simple with newChangeFlusher", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		treeStorage := ctx.treeStorage
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree.(*objectTree)
		objTree.flusher = &newChangeFlusher{}

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

		res, err := objTree.AddRawChangesWithUpdater(context.Background(), payload, func(tree ObjectTree, md Mode) error {
			// check tree iterate
			var iterChangesId []string
			err := objTree.IterateRoot(nil, func(change *Change) bool {
				iterChangesId = append(iterChangesId, change.Id)
				return true
			})
			require.NoError(t, err, "iterate should be without error")
			assert.Equal(t, []string{"0", "1", "2", "3", "4"}, iterChangesId)
			assert.Equal(t, "0", objTree.Root().Id)

			for _, ch := range rawChanges {
				treeCh, err := objTree.GetChange(ch.Id)
				require.NoError(t, err)
				require.True(t, treeCh.IsNew)
			}
			return nil
		})
		require.NoError(t, err, "adding changes should be without error")

		// check result
		assert.Equal(t, []string{"0"}, res.OldHeads)
		assert.Equal(t, []string{"4"}, res.Heads)
		assert.Equal(t, len(rawChanges), len(res.Added))
		assert.Equal(t, Append, res.Mode)

		// check tree heads
		assert.Equal(t, []string{"4"}, objTree.Heads())

		// check storage
		heads, _ := treeStorage.Heads()
		assert.Equal(t, []string{"4"}, heads)

		// after Flush
		assert.Equal(t, "3", objTree.Root().Id)
		for _, ch := range rawChanges {
			raw, err := treeStorage.GetRawChange(context.Background(), ch.Id)
			assert.NoError(t, err, "storage should have all the changes")
			assert.Equal(t, ch, raw, "the changes in the storage should be the same")
			treeCh, err := objTree.GetChange(ch.Id)
			if ch.Id == "3" || ch.Id == "4" {
				require.NoError(t, err)
				require.False(t, treeCh.IsNew)
				continue
			}
			require.Error(t, err)
		}
	})

	t.Run("update failed, nothing saved", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		treeStorage := ctx.treeStorage
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree.(*objectTree)
		objTree.flusher = &newChangeFlusher{}

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

		_, err := objTree.AddRawChangesWithUpdater(context.Background(), payload, func(tree ObjectTree, md Mode) error {
			return fmt.Errorf("some error")
		})
		require.Equal(t, fmt.Errorf("some error"), err)

		// check tree heads
		require.Equal(t, []string{"0"}, objTree.Heads())

		// check storage
		heads, _ := treeStorage.Heads()
		require.Equal(t, []string{"0"}, heads)

		require.Equal(t, "0", objTree.Root().Id)
		for _, ch := range rawChanges {
			_, err := treeStorage.GetRawChange(context.Background(), ch.Id)
			require.Error(t, err)
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

	t.Run("test tree filter", func(t *testing.T) {
		filterFunc := func(change *Change) bool {
			return slices.Contains([]string{"0", "1", "2", "3", "4", "7", "8"}, change.Id)
		}
		ctx := prepareContext(t, aclList, genBuildFilterableTestableTree(filterFunc), false, nil)
		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			ctx.changeCreator.CreateRawWithData("1", aclList.Head().Id, "0", false, []byte("1"), "0"),
			ctx.changeCreator.CreateRawWithData("2", aclList.Head().Id, "0", false, []byte("2"), "1"),
			ctx.changeCreator.CreateRawWithData("3", aclList.Head().Id, "0", false, []byte("3"), "2"),
			ctx.changeCreator.CreateRawWithData("4", aclList.Head().Id, "0", false, []byte("4"), "2"),
			ctx.changeCreator.CreateRawWithData("5", aclList.Head().Id, "0", false, []byte("5"), "1"),
			ctx.changeCreator.CreateRawWithData("6", aclList.Head().Id, "0", true, []byte("6"), "3", "4", "5"),
			ctx.changeCreator.CreateRawWithData("7", aclList.Head().Id, "6", false, []byte("7"), "6"),
			ctx.changeCreator.CreateRawWithData("8", aclList.Head().Id, "6", false, []byte("8"), "6"),
		}
		_, err := ctx.objTree.AddRawChanges(context.Background(), RawChangesPayload{
			NewHeads:   []string{"7", "8"},
			RawChanges: rawChanges,
		})
		require.NoError(t, err)
		var ids []string
		ctx.objTree.IterateRoot(nil, func(change *Change) bool {
			ids = append(ids, change.Id)
			return true
		})
		slices.Sort(ids)
		require.Equal(t, []string{"0", "1", "2", "3", "4"}, ids)
	})

	t.Run("test tree restore", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			ctx.changeCreator.CreateRawWithData("1", aclList.Head().Id, "0", false, []byte("1"), "0"),
			ctx.changeCreator.CreateRawWithData("2", aclList.Head().Id, "0", false, []byte("2"), "1"),
			ctx.changeCreator.CreateRawWithData("3", aclList.Head().Id, "0", false, []byte("3"), "2"),
			ctx.changeCreator.CreateRawWithData("4", aclList.Head().Id, "0", false, []byte("4"), "2"),
			ctx.changeCreator.CreateRawWithData("5", aclList.Head().Id, "0", false, []byte("5"), "1"),
			ctx.changeCreator.CreateRawWithData("6", aclList.Head().Id, "0", true, []byte("6"), "3", "4", "5"),
			ctx.changeCreator.CreateRawWithData("7", aclList.Head().Id, "6", false, []byte("7"), "6"),
			ctx.changeCreator.CreateRawWithData("8", aclList.Head().Id, "6", false, []byte("8"), "6"),
		}
		_, err := ctx.objTree.AddRawChanges(context.Background(), RawChangesPayload{
			NewHeads:   []string{"7", "8"},
			RawChanges: rawChanges,
		})
		require.NoError(t, err)
		st := ctx.treeStorage.(*treestorage.InMemoryTreeStorage)
		st.Remove("6")
		newCh := ctx.changeCreator.CreateRawWithData("9", aclList.Head().Id, "0", false, []byte("9"), "3", "4", "5")
		_, err = ctx.objTree.AddRawChanges(context.Background(), RawChangesPayload{
			NewHeads:   []string{"9"},
			RawChanges: []*treechangeproto.RawTreeChangeWithId{newCh},
		})
		require.NoError(t, err)
		require.Equal(t, []string{"9"}, ctx.objTree.Heads())
		heads, err := ctx.objTree.Storage().Heads()
		require.NoError(t, err)
		require.Equal(t, []string{"9"}, heads)
	})

	t.Run("rollback when add to storage returns error", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree
		store := ctx.treeStorage.(*treestorage.InMemoryTreeStorage)
		addErr := fmt.Errorf("error saving")
		store.SetReturnErrorOnAdd(addErr)

		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("1", aclList.Head().Id, "0", true, "0"),
		}
		payload := RawChangesPayload{
			NewHeads:   []string{"1"},
			RawChanges: rawChanges,
		}
		_, err := objTree.AddRawChanges(context.Background(), payload)
		require.Error(t, err, addErr)
		require.Equal(t, "0", objTree.Root().Id)
	})

	t.Run("their heads before common snapshot", func(t *testing.T) {
		// checking that adding old changes did not affect the tree
		ctx := prepareTreeContext(t, aclList)
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree

		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("1", aclList.Head().Id, "0", true, "0"),
			changeCreator.CreateRaw("2", aclList.Head().Id, "1", false, "1"),
			changeCreator.CreateRaw("3", aclList.Head().Id, "1", true, "2"),
			changeCreator.CreateRaw("4", aclList.Head().Id, "1", false, "2"),
			changeCreator.CreateRaw("5", aclList.Head().Id, "1", false, "1"),
			changeCreator.CreateRaw("6", aclList.Head().Id, "1", true, "3", "4", "5"),
		}
		payload := RawChangesPayload{
			NewHeads:   []string{rawChanges[len(rawChanges)-1].Id},
			RawChanges: rawChanges,
		}
		_, err := objTree.AddRawChanges(context.Background(), payload)
		require.NoError(t, err, "adding changes should be without error")
		require.Equal(t, "6", objTree.Root().Id)

		rawChangesPrevious := []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("1", aclList.Head().Id, "0", true, "0"),
		}
		payload = RawChangesPayload{
			NewHeads:   []string{"1"},
			RawChanges: rawChangesPrevious,
		}
		_, err = objTree.AddRawChanges(context.Background(), payload)
		require.NoError(t, err, "adding changes should be without error")
		require.Equal(t, "6", objTree.Root().Id)
	})

	t.Run("stored changes will not break the pipeline if heads were not updated", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree
		store := ctx.treeStorage.(*treestorage.InMemoryTreeStorage)

		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("1", aclList.Head().Id, "0", true, "0"),
		}
		payload := RawChangesPayload{
			NewHeads:   []string{rawChanges[len(rawChanges)-1].Id},
			RawChanges: rawChanges,
		}
		_, err := objTree.AddRawChanges(context.Background(), payload)
		require.NoError(t, err, "adding changes should be without error")
		require.Equal(t, "1", objTree.Root().Id)

		// creating changes to save in the storage
		// to imitate the condition where all changes are in the storage
		// but the head was not updated
		storageChanges := []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("2", aclList.Head().Id, "1", false, "1"),
			changeCreator.CreateRaw("3", aclList.Head().Id, "1", true, "2"),
			changeCreator.CreateRaw("4", aclList.Head().Id, "1", false, "2"),
			changeCreator.CreateRaw("5", aclList.Head().Id, "1", false, "1"),
			changeCreator.CreateRaw("6", aclList.Head().Id, "1", true, "3", "4", "5"),
		}
		store.AddRawChangesSetHeads(storageChanges, []string{"1"})

		// updating with subset of those changes to see that everything will still work
		payload = RawChangesPayload{
			NewHeads:   []string{"6"},
			RawChanges: storageChanges,
		}
		_, err = objTree.AddRawChanges(context.Background(), payload)
		require.NoError(t, err, "adding changes should be without error")
		require.Equal(t, "6", objTree.Root().Id)
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

			for _, rawCh := range changes {
				ch, err := ctx.objTree.(*objectTree).changeBuilder.Unmarshall(rawCh, false)
				require.NoError(t, err)
				require.Equal(t, mockDataType, ch.DataType)
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
		deps.treeStorage.AddRawChangesSetHeads(rawChanges, []string{"6"})
		hTree, err := buildHistoryTree(deps, HistoryTreeParams{
			Heads:           []string{"6"},
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

	t.Run("test history tree build full", func(t *testing.T) {
		changeCreator, deps := prepareHistoryTreeDeps(aclList)

		// sequence of snapshots: 5->1->0
		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			changeCreator.CreateRaw("1", aclList.Head().Id, "0", true, "0"),
			changeCreator.CreateRaw("2", aclList.Head().Id, "1", false, "1"),
			changeCreator.CreateRaw("3", aclList.Head().Id, "1", true, "2"),
			changeCreator.CreateRaw("4", aclList.Head().Id, "1", false, "2"),
			changeCreator.CreateRaw("5", aclList.Head().Id, "1", true, "3", "4"),
			changeCreator.CreateRaw("6", aclList.Head().Id, "5", false, "5"),
		}
		deps.treeStorage.AddRawChangesSetHeads(rawChanges, []string{"6"})
		hTree, err := buildHistoryTree(deps, HistoryTreeParams{})
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
		deps.treeStorage.AddRawChangesSetHeads(rawChanges, []string{"6"})
		hTree, err := buildHistoryTree(deps, HistoryTreeParams{
			Heads: []string{"6"}, IncludeBeforeId: true,
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
			Heads:           []string{"0"},
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

	t.Run("validate tree", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		changeCreator := ctx.changeCreator

		rawChanges := []*treechangeproto.RawTreeChangeWithId{
			ctx.objTree.Header(),
			changeCreator.CreateRaw("1", aclList.Head().Id, "0", false, "0"),
			changeCreator.CreateRaw("2", aclList.Head().Id, "0", false, "1"),
			changeCreator.CreateRaw("3", aclList.Head().Id, "0", true, "2"),
		}
		emptyDataTreeDeps = nonVerifiableTreeDeps
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
		emptyDataTreeDeps = nonVerifiableTreeDeps
		err := ValidateRawTree(treestorage.TreeStorageCreatePayload{
			RootRawChange: ctx.objTree.Header(),
			Heads:         []string{"3"},
			Changes:       rawChanges,
		}, ctx.aclList)
		require.True(t, errors.Is(err, ErrHasInvalidChanges))
	})

	t.Run("gen changes test load iterator simple", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree
		result := genChanges(changeCreator, genParams{
			prefix:     "id",
			aclId:      aclList.Id(),
			startIdx:   0,
			levels:     100,
			perLevel:   10,
			snapshotId: objTree.Root().Id,
			prevHeads:  []string{objTree.Root().Id},
			isSnapshot: func() bool {
				return false
			},
			hasData: false,
		})
		_, err := objTree.AddRawChanges(context.Background(), RawChangesPayload{
			NewHeads:   result.heads,
			RawChanges: result.changes,
		})
		require.NoError(t, err)
		iter, err := objTree.ChangesAfterCommonSnapshotLoader([]string{objTree.Id()}, []string{objTree.Id()})
		require.NoError(t, err)
		otherTreeStorage := changeCreator.CreateNewTreeStorage("0", aclList.Head().Id, false)
		otherTree, err := BuildTestableTree(otherTreeStorage, aclList)
		require.NoError(t, err)
		for {
			batch, err := iter.NextBatch(300)
			require.NoError(t, err)
			if len(batch.Batch) == 0 {
				break
			}
			res, err := otherTree.AddRawChanges(context.Background(), RawChangesPayload{
				NewHeads:   batch.Heads,
				RawChanges: batch.Batch,
			})
			require.NoError(t, err)
			require.Equal(t, len(batch.Batch), len(res.Added))
		}
		require.Equal(t, objTree.Heads(), otherTree.Heads())
	})

	t.Run("gen changes test load iterator each change exceed max size", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree
		result := genChanges(changeCreator, genParams{
			prefix:     "id",
			aclId:      aclList.Id(),
			startIdx:   0,
			levels:     100,
			perLevel:   10,
			snapshotId: objTree.Root().Id,
			prevHeads:  []string{objTree.Root().Id},
			isSnapshot: func() bool {
				return false
			},
			hasData: false,
		})
		_, err := objTree.AddRawChanges(context.Background(), RawChangesPayload{
			NewHeads:   result.heads,
			RawChanges: result.changes,
		})
		require.NoError(t, err)
		iter, err := objTree.ChangesAfterCommonSnapshotLoader([]string{objTree.Id()}, []string{objTree.Id()})
		require.NoError(t, err)
		otherTreeStorage := changeCreator.CreateNewTreeStorage("0", aclList.Head().Id, false)
		otherTree, err := BuildTestableTree(otherTreeStorage, aclList)
		require.NoError(t, err)
		for {
			batch, err := iter.NextBatch(1)
			require.NoError(t, err)
			if len(batch.Batch) == 0 {
				break
			}
			res, err := otherTree.AddRawChanges(context.Background(), RawChangesPayload{
				NewHeads:   batch.Heads,
				RawChanges: batch.Batch,
			})
			require.NoError(t, err)
			require.Equal(t, len(batch.Batch), len(res.Added))
		}
		require.Equal(t, objTree.Heads(), otherTree.Heads())
	})

	t.Run("gen changes test load iterator snapshots", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree
		snapshotCounter := 0
		result := genChanges(changeCreator, genParams{
			prefix:     "id",
			aclId:      aclList.Id(),
			startIdx:   0,
			levels:     100,
			perLevel:   10,
			snapshotId: objTree.Root().Id,
			prevHeads:  []string{objTree.Root().Id},
			isSnapshot: func() bool {
				snapshotCounter++
				return snapshotCounter%10 == 0
			},
			hasData: false,
		})
		_, err := objTree.AddRawChanges(context.Background(), RawChangesPayload{
			NewHeads:   result.heads,
			RawChanges: result.changes,
		})
		require.NoError(t, err)
		iter, err := objTree.ChangesAfterCommonSnapshotLoader([]string{objTree.Id()}, []string{objTree.Id()})
		require.NoError(t, err)
		otherTreeStorage := changeCreator.CreateNewTreeStorage("0", aclList.Head().Id, false)
		otherTree, err := BuildTestableTree(otherTreeStorage, aclList)
		require.NoError(t, err)
		for {
			batch, err := iter.NextBatch(300)
			require.NoError(t, err)
			if len(batch.Batch) == 0 {
				break
			}
			res, err := otherTree.AddRawChanges(context.Background(), RawChangesPayload{
				NewHeads:   batch.Heads,
				RawChanges: batch.Batch,
			})
			require.NoError(t, err)
			require.Equal(t, len(batch.Batch), len(res.Added))
		}
		require.Equal(t, objTree.Heads(), otherTree.Heads())
	})

	t.Run("gen changes test load iterator non empty other tree", func(t *testing.T) {
		ctx := prepareTreeContext(t, aclList)
		changeCreator := ctx.changeCreator
		objTree := ctx.objTree
		snapshotCounter := 0
		result := genChanges(changeCreator, genParams{
			prefix:     "id",
			aclId:      aclList.Id(),
			startIdx:   0,
			levels:     93,
			perLevel:   10,
			snapshotId: objTree.Root().Id,
			prevHeads:  []string{objTree.Root().Id},
			isSnapshot: func() bool {
				snapshotCounter++
				return snapshotCounter%50 == 0
			},
			hasData: false,
		})
		otherTreeStorage := changeCreator.CreateNewTreeStorage("0", aclList.Head().Id, false)
		otherTree, err := BuildTestableTree(otherTreeStorage, aclList)
		_, err = objTree.AddRawChanges(context.Background(), RawChangesPayload{
			NewHeads:   result.heads,
			RawChanges: result.changes,
		})
		require.NoError(t, err)
		_, err = otherTree.AddRawChanges(context.Background(), RawChangesPayload{
			NewHeads:   result.heads,
			RawChanges: result.changes,
		})
		require.NoError(t, err)
		result = genChanges(changeCreator, genParams{
			prefix:     "id",
			aclId:      aclList.Id(),
			startIdx:   93,
			levels:     29,
			perLevel:   10,
			snapshotId: objTree.Root().Id,
			prevHeads:  objTree.Heads(),
			isSnapshot: func() bool {
				snapshotCounter++
				return snapshotCounter%50 == 0
			},
			hasData: false,
		})
		_, err = objTree.AddRawChanges(context.Background(), RawChangesPayload{
			NewHeads:   result.heads,
			RawChanges: result.changes,
		})
		require.NoError(t, err)
		iter, err := objTree.ChangesAfterCommonSnapshotLoader(otherTree.SnapshotPath(), otherTree.Heads())
		require.NoError(t, err)
		for {
			batch, err := iter.NextBatch(400)
			require.NoError(t, err)
			if len(batch.Batch) == 0 {
				break
			}
			res, err := otherTree.AddRawChanges(context.Background(), RawChangesPayload{
				NewHeads:   batch.Heads,
				RawChanges: batch.Batch,
			})
			require.NoError(t, err)
			require.Equal(t, len(batch.Batch), len(res.Added))
		}
		require.Equal(t, objTree.Heads(), otherTree.Heads())
	})
}

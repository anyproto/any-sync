package objecttree

import (
	"context"
	"fmt"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/util/crypto"
)

type ObjectTreeCreatePayload struct {
	PrivKey       crypto.PrivKey
	ChangeType    string
	ChangePayload []byte
	SpaceId       string
	IsEncrypted   bool
	Seed          []byte
	Timestamp     int64
}

type ObjectTreeDerivePayload struct {
	ChangeType    string
	ChangePayload []byte
	SpaceId       string
	IsEncrypted   bool
}

type HistoryTreeParams struct {
	Storage         Storage
	AclList         list.AclList
	Heads           []string
	IncludeBeforeId bool
}

type objectTreeDeps struct {
	changeBuilder ChangeBuilder
	treeBuilder   *treeBuilder
	storage       Storage
	validator     ObjectTreeValidator
	aclList       list.AclList
	flusher       Flusher
}

type BuildObjectTreeFunc = func(storage Storage, aclList list.AclList) (ObjectTree, error)

var defaultObjectTreeDeps = verifiableTreeDeps

func verifiableTreeDeps(
	rootChange *treechangeproto.RawTreeChangeWithId,
	storage Storage,
	aclList list.AclList) objectTreeDeps {
	changeBuilder := NewChangeBuilder(crypto.NewKeyStorage(), rootChange)
	treeBuilder := newTreeBuilder(storage, changeBuilder)
	return objectTreeDeps{
		changeBuilder: changeBuilder,
		treeBuilder:   treeBuilder,
		storage:       storage,
		validator:     newTreeValidator(false, false),
		aclList:       aclList,
		flusher:       &defaultFlusher{},
	}
}

var emptyDataTreeDeps = verifiableEmptyDataTreeDeps

func verifiableEmptyDataTreeDeps(
	rootChange *treechangeproto.RawTreeChangeWithId,
	storage Storage,
	aclList list.AclList) objectTreeDeps {
	changeBuilder := NewEmptyDataChangeBuilder(crypto.NewKeyStorage(), rootChange)
	treeBuilder := newTreeBuilder(storage, changeBuilder)
	return objectTreeDeps{
		changeBuilder: changeBuilder,
		treeBuilder:   treeBuilder,
		storage:       storage,
		validator:     newTreeValidator(false, false),
		aclList:       aclList,
		flusher:       &defaultFlusher{},
	}
}

func nonVerifiableEmptyDataTreeDeps(
	rootChange *treechangeproto.RawTreeChangeWithId,
	storage Storage,
	aclList list.AclList) objectTreeDeps {
	changeBuilder := &nonVerifiableChangeBuilder{NewEmptyDataChangeBuilder(crypto.NewKeyStorage(), rootChange)}
	treeBuilder := newTreeBuilder(storage, changeBuilder)
	return objectTreeDeps{
		changeBuilder: changeBuilder,
		treeBuilder:   treeBuilder,
		storage:       storage,
		validator:     newTreeValidator(false, false),
		aclList:       aclList,
		flusher:       &defaultFlusher{},
	}
}

func nonVerifiableTreeDeps(
	rootChange *treechangeproto.RawTreeChangeWithId,
	storage Storage,
	aclList list.AclList) objectTreeDeps {
	changeBuilder := &nonVerifiableChangeBuilder{NewChangeBuilder(newMockKeyStorage(), rootChange)}
	treeBuilder := newTreeBuilder(storage, changeBuilder)
	return objectTreeDeps{
		changeBuilder: changeBuilder,
		treeBuilder:   treeBuilder,
		storage:       storage,
		validator:     &noOpTreeValidator{},
		aclList:       aclList,
		flusher:       &defaultFlusher{},
	}
}

func BuildEmptyDataObjectTree(storage Storage, aclList list.AclList) (ObjectTree, error) {
	rootChange, err := storage.Root(context.Background())
	if err != nil {
		return nil, err
	}
	deps := emptyDataTreeDeps(rootChange.RawTreeChangeWithId(), storage, aclList)
	return buildObjectTree(deps)
}

func BuildTestableTree(storage Storage, aclList list.AclList) (ObjectTree, error) {
	root, _ := storage.Root(context.Background())
	changeBuilder := &nonVerifiableChangeBuilder{
		ChangeBuilder: NewChangeBuilder(newMockKeyStorage(), root.RawTreeChangeWithId()),
	}
	deps := objectTreeDeps{
		changeBuilder: changeBuilder,
		treeBuilder:   newTreeBuilder(storage, changeBuilder),
		storage:       storage,
		validator:     &noOpTreeValidator{},
		aclList:       aclList,
		flusher:       &defaultFlusher{},
	}

	return buildObjectTree(deps)
}

func BuildEmptyDataTestableTree(storage Storage, aclList list.AclList) (ObjectTree, error) {
	root, _ := storage.Root(context.Background())
	changeBuilder := &nonVerifiableChangeBuilder{
		ChangeBuilder: NewEmptyDataChangeBuilder(newMockKeyStorage(), root.RawTreeChangeWithId()),
	}
	deps := objectTreeDeps{
		changeBuilder: changeBuilder,
		treeBuilder:   newTreeBuilder(storage, changeBuilder),
		storage:       storage,
		validator:     &noOpTreeValidator{},
		aclList:       aclList,
		flusher:       &defaultFlusher{},
	}

	return buildObjectTree(deps)
}

func BuildMigratableObjectTree(storage Storage, aclList list.AclList) (ObjectTree, error) {
	rootChange, err := storage.Root(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get root: %w", err)
	}
	deps := nonVerifiableEmptyDataTreeDeps(rootChange.RawTreeChangeWithId(), storage, aclList)
	return buildObjectTree(deps)
}

func BuildKeyFilterableObjectTree(storage Storage, aclList list.AclList) (ObjectTree, error) {
	rootChange, err := storage.Root(context.Background())
	if err != nil {
		return nil, err
	}
	deps := defaultObjectTreeDeps(rootChange.RawTreeChangeWithId(), storage, aclList)
	deps.validator = newTreeValidator(true, true)
	return buildObjectTree(deps)
}

func BuildEmptyDataKeyFilterableObjectTree(storage Storage, aclList list.AclList) (ObjectTree, error) {
	rootChange, err := storage.Root(context.Background())
	if err != nil {
		return nil, err
	}
	deps := emptyDataTreeDeps(rootChange.RawTreeChangeWithId(), storage, aclList)
	deps.validator = newTreeValidator(true, true)
	return buildObjectTree(deps)
}

func BuildObjectTree(storage Storage, aclList list.AclList) (ObjectTree, error) {
	rootChange, err := storage.Root(context.Background())
	if err != nil {
		return nil, err
	}
	deps := defaultObjectTreeDeps(rootChange.RawTreeChangeWithId(), storage, aclList)
	return buildObjectTree(deps)
}

func BuildNonVerifiableHistoryTree(params HistoryTreeParams) (HistoryTree, error) {
	rootChange, err := params.Storage.Root(context.Background())
	if err != nil {
		return nil, err
	}
	deps := nonVerifiableTreeDeps(rootChange.RawTreeChangeWithId(), params.Storage, params.AclList)
	return buildHistoryTree(deps, params)
}

func BuildHistoryTree(params HistoryTreeParams) (HistoryTree, error) {
	rootChange, err := params.Storage.Root(context.Background())
	if err != nil {
		return nil, err
	}
	deps := defaultObjectTreeDeps(rootChange.RawTreeChangeWithId(), params.Storage, params.AclList)
	return buildHistoryTree(deps, params)
}

func CreateObjectTreeRoot(payload ObjectTreeCreatePayload, aclList list.AclList) (root *treechangeproto.RawTreeChangeWithId, err error) {
	aclList.RLock()
	aclHeadId := aclList.Head().Id
	aclList.RUnlock()
	cnt := InitialContent{
		AclHeadId:     aclHeadId,
		PrivKey:       payload.PrivKey,
		SpaceId:       payload.SpaceId,
		ChangeType:    payload.ChangeType,
		ChangePayload: payload.ChangePayload,
		Timestamp:     payload.Timestamp,
		Seed:          payload.Seed,
	}

	_, root, err = NewChangeBuilder(crypto.NewKeyStorage(), nil).BuildRoot(cnt)
	return
}

func DeriveObjectTreeRoot(payload ObjectTreeDerivePayload, aclList list.AclList) (root *treechangeproto.RawTreeChangeWithId, err error) {
	cnt := InitialDerivedContent{
		SpaceId:       payload.SpaceId,
		ChangeType:    payload.ChangeType,
		ChangePayload: payload.ChangePayload,
	}
	_, root, err = NewChangeBuilder(crypto.NewKeyStorage(), nil).BuildDerivedRoot(cnt)
	return
}

func buildObjectTree(deps objectTreeDeps) (ObjectTree, error) {
	objTree := &objectTree{
		id:              deps.storage.Id(),
		storage:         deps.storage,
		treeBuilder:     deps.treeBuilder,
		validator:       deps.validator,
		aclList:         deps.aclList,
		changeBuilder:   deps.changeBuilder,
		keys:            make(map[string]crypto.SymKey),
		newChangesBuf:   make([]*Change, 0, 10),
		difSnapshotBuf:  make([]*treechangeproto.RawTreeChangeWithId, 0, 10),
		notSeenIdxBuf:   make([]int, 0, 10),
		newSnapshotsBuf: make([]*Change, 0, 10),
		flusher:         deps.flusher,
	}

	err := objTree.rebuildFromStorage(nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to rebuild from storage: %w", err)
	}

	// TODO: think about contexts
	root, err := objTree.storage.Root(context.Background())
	if err != nil {
		return nil, err
	}
	objTree.rawRoot = root.RawTreeChangeWithId()

	// verifying root
	header, err := objTree.changeBuilder.Unmarshall(objTree.rawRoot, true)
	if err != nil {
		return nil, err
	}
	objTree.root = header

	return objTree, nil
}

func buildHistoryTree(deps objectTreeDeps, params HistoryTreeParams) (ht HistoryTree, err error) {
	objTree := &objectTree{
		id:              deps.storage.Id(),
		storage:         deps.storage,
		treeBuilder:     deps.treeBuilder,
		validator:       deps.validator,
		aclList:         deps.aclList,
		changeBuilder:   deps.changeBuilder,
		keys:            make(map[string]crypto.SymKey),
		newChangesBuf:   make([]*Change, 0, 10),
		difSnapshotBuf:  make([]*treechangeproto.RawTreeChangeWithId, 0, 10),
		notSeenIdxBuf:   make([]int, 0, 10),
		newSnapshotsBuf: make([]*Change, 0, 10),
		flusher:         deps.flusher,
	}

	hTree := &historyTree{objectTree: objTree}
	err = hTree.rebuildFromStorage(params)
	if err != nil {
		return nil, err
	}
	objTree.id = objTree.storage.Id()
	root, err := objTree.storage.Root(context.Background())
	if err != nil {
		return nil, err
	}
	objTree.rawRoot = root.RawTreeChangeWithId()

	header, err := objTree.changeBuilder.Unmarshall(objTree.rawRoot, false)
	if err != nil {
		return nil, err
	}
	objTree.root = header
	return hTree, nil
}

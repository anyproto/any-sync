package objecttree

import (
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
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
	TreeStorage     treestorage.TreeStorage
	AclList         list.AclList
	Heads           []string
	IncludeBeforeId bool
}

type objectTreeDeps struct {
	changeBuilder   ChangeBuilder
	treeBuilder     *treeBuilder
	treeStorage     treestorage.TreeStorage
	validator       ObjectTreeValidator
	rawChangeLoader *rawChangeLoader
	aclList         list.AclList
	flusher         Flusher
}

type BuildObjectTreeFunc = func(treeStorage treestorage.TreeStorage, aclList list.AclList) (ObjectTree, error)

var defaultObjectTreeDeps = verifiableTreeDeps

func verifiableTreeDeps(
	rootChange *treechangeproto.RawTreeChangeWithId,
	treeStorage treestorage.TreeStorage,
	aclList list.AclList) objectTreeDeps {
	changeBuilder := NewChangeBuilder(crypto.NewKeyStorage(), rootChange)
	rawLoader := newRawChangeLoader(treeStorage, changeBuilder)
	treeBuilder := newTreeBuilder(true, treeStorage, changeBuilder, rawLoader)
	return objectTreeDeps{
		changeBuilder:   changeBuilder,
		treeBuilder:     treeBuilder,
		treeStorage:     treeStorage,
		validator:       newTreeValidator(false, false),
		rawChangeLoader: rawLoader,
		aclList:         aclList,
		flusher:         &defaultFlusher{},
	}
}

var emptyDataTreeDeps = verifiableEmptyDataTreeDeps

func verifiableEmptyDataTreeDeps(
	rootChange *treechangeproto.RawTreeChangeWithId,
	treeStorage treestorage.TreeStorage,
	aclList list.AclList) objectTreeDeps {
	changeBuilder := NewEmptyDataChangeBuilder(crypto.NewKeyStorage(), rootChange)
	loader := newStorageLoader(treeStorage, changeBuilder)
	treeBuilder := newTreeBuilder(false, treeStorage, changeBuilder, loader)
	return objectTreeDeps{
		changeBuilder:   changeBuilder,
		treeBuilder:     treeBuilder,
		treeStorage:     treeStorage,
		validator:       newTreeValidator(false, false),
		rawChangeLoader: loader,
		aclList:         aclList,
		flusher:         &defaultFlusher{},
	}
}

func nonVerifiableTreeDeps(
	rootChange *treechangeproto.RawTreeChangeWithId,
	treeStorage treestorage.TreeStorage,
	aclList list.AclList) objectTreeDeps {
	changeBuilder := &nonVerifiableChangeBuilder{NewChangeBuilder(newMockKeyStorage(), rootChange)}
	loader := newRawChangeLoader(treeStorage, changeBuilder)
	treeBuilder := newTreeBuilder(true, treeStorage, changeBuilder, loader)
	return objectTreeDeps{
		changeBuilder:   changeBuilder,
		treeBuilder:     treeBuilder,
		treeStorage:     treeStorage,
		validator:       &noOpTreeValidator{},
		rawChangeLoader: loader,
		aclList:         aclList,
		flusher:         &defaultFlusher{},
	}
}

func BuildEmptyDataObjectTree(treeStorage treestorage.TreeStorage, aclList list.AclList) (ObjectTree, error) {
	rootChange, err := treeStorage.Root()
	if err != nil {
		return nil, err
	}
	deps := emptyDataTreeDeps(rootChange, treeStorage, aclList)
	return buildObjectTree(deps)
}

func BuildTestableTree(treeStorage treestorage.TreeStorage, aclList list.AclList) (ObjectTree, error) {
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
		flusher:         &defaultFlusher{},
	}

	return buildObjectTree(deps)
}

func BuildEmptyDataTestableTree(treeStorage treestorage.TreeStorage, aclList list.AclList) (ObjectTree, error) {
	root, _ := treeStorage.Root()
	changeBuilder := &nonVerifiableChangeBuilder{
		ChangeBuilder: NewEmptyDataChangeBuilder(newMockKeyStorage(), root),
	}
	loader := newStorageLoader(treeStorage, changeBuilder)
	deps := objectTreeDeps{
		changeBuilder:   changeBuilder,
		treeBuilder:     newTreeBuilder(false, treeStorage, changeBuilder, loader),
		treeStorage:     treeStorage,
		rawChangeLoader: loader,
		validator:       &noOpTreeValidator{},
		aclList:         aclList,
		flusher:         &defaultFlusher{},
	}

	return buildObjectTree(deps)
}

func BuildKeyFilterableObjectTree(treeStorage treestorage.TreeStorage, aclList list.AclList) (ObjectTree, error) {
	rootChange, err := treeStorage.Root()
	if err != nil {
		return nil, err
	}
	deps := defaultObjectTreeDeps(rootChange, treeStorage, aclList)
	deps.validator = newTreeValidator(true, true)
	return buildObjectTree(deps)
}

func BuildEmptyDataKeyFilterableObjectTree(treeStorage treestorage.TreeStorage, aclList list.AclList) (ObjectTree, error) {
	rootChange, err := treeStorage.Root()
	if err != nil {
		return nil, err
	}
	deps := emptyDataTreeDeps(rootChange, treeStorage, aclList)
	deps.validator = newTreeValidator(true, true)
	return buildObjectTree(deps)
}

func BuildObjectTree(treeStorage treestorage.TreeStorage, aclList list.AclList) (ObjectTree, error) {
	rootChange, err := treeStorage.Root()
	if err != nil {
		return nil, err
	}
	deps := defaultObjectTreeDeps(rootChange, treeStorage, aclList)
	return buildObjectTree(deps)
}

func BuildNonVerifiableHistoryTree(params HistoryTreeParams) (HistoryTree, error) {
	rootChange, err := params.TreeStorage.Root()
	if err != nil {
		return nil, err
	}
	deps := nonVerifiableTreeDeps(rootChange, params.TreeStorage, params.AclList)
	return buildHistoryTree(deps, params)
}

func BuildHistoryTree(params HistoryTreeParams) (HistoryTree, error) {
	rootChange, err := params.TreeStorage.Root()
	if err != nil {
		return nil, err
	}
	deps := defaultObjectTreeDeps(rootChange, params.TreeStorage, params.AclList)
	return buildHistoryTree(deps, params)
}

func CreateObjectTreeRoot(payload ObjectTreeCreatePayload, aclList list.AclList) (root *treechangeproto.RawTreeChangeWithId, err error) {
	aclList.RLock()
	aclHeadId := aclList.Head().Id
	aclList.RUnlock()

	if err != nil {
		return
	}
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
		id:              deps.treeStorage.Id(),
		treeStorage:     deps.treeStorage,
		treeBuilder:     deps.treeBuilder,
		validator:       deps.validator,
		aclList:         deps.aclList,
		changeBuilder:   deps.changeBuilder,
		rawChangeLoader: deps.rawChangeLoader,
		keys:            make(map[string]crypto.SymKey),
		newChangesBuf:   make([]*Change, 0, 10),
		difSnapshotBuf:  make([]*treechangeproto.RawTreeChangeWithId, 0, 10),
		notSeenIdxBuf:   make([]int, 0, 10),
		newSnapshotsBuf: make([]*Change, 0, 10),
		flusher:         deps.flusher,
	}

	err := objTree.rebuildFromStorage(nil, nil, nil)
	if err != nil {
		return nil, err
	}

	objTree.rawRoot, err = objTree.treeStorage.Root()
	if err != nil {
		return nil, err
	}

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
		id:              deps.treeStorage.Id(),
		treeStorage:     deps.treeStorage,
		treeBuilder:     deps.treeBuilder,
		validator:       deps.validator,
		aclList:         deps.aclList,
		changeBuilder:   deps.changeBuilder,
		rawChangeLoader: deps.rawChangeLoader,
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
	objTree.id = objTree.treeStorage.Id()
	objTree.rawRoot, err = objTree.treeStorage.Root()
	if err != nil {
		return nil, err
	}

	header, err := objTree.changeBuilder.Unmarshall(objTree.rawRoot, false)
	if err != nil {
		return nil, err
	}
	objTree.root = header
	return hTree, nil
}

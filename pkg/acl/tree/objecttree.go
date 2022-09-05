package tree

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"sync"
	"time"
)

type ObjectTreeUpdateListener interface {
	Update(tree ObjectTree)
	Rebuild(tree ObjectTree)
}

type RWLocker interface {
	sync.Locker
	RLock()
	RUnlock()
}

var (
	ErrHasInvalidChanges = errors.New("the change is invalid")
	ErrNoCommonSnapshot  = errors.New("trees doesn't have a common snapshot")
)

type AddResultSummary int

const (
	AddResultSummaryNothing AddResultSummary = iota
	AddResultSummaryAppend
	AddResultSummaryRebuild
)

type AddResult struct {
	OldHeads []string
	Heads    []string
	Added    []*aclpb.RawChange

	Summary AddResultSummary
}

type ObjectTree interface {
	RWLocker

	ID() string
	Header() *aclpb.Header
	Heads() []string
	Root() *Change
	HasChange(string) bool

	Iterate(func(change *Change) bool)
	IterateFrom(string, func(change *Change) bool)

	SnapshotPath() []string
	ChangesAfterCommonSnapshot(snapshotPath []string) ([]*aclpb.RawChange, error)

	Storage() storage.TreeStorage
	DebugDump() (string, error)

	AddContent(ctx context.Context, aclList list.ACLList, content SignableChangeContent) (*aclpb.RawChange, error)
	AddRawChanges(ctx context.Context, aclList list.ACLList, changes ...*aclpb.RawChange) (AddResult, error)

	Close() error
}

type objectTree struct {
	treeStorage    storage.TreeStorage
	updateListener ObjectTreeUpdateListener

	id     string
	header *aclpb.Header
	tree   *Tree

	treeBuilder *treeBuilder
	validator   DocTreeValidator
	kch         *keychain

	// buffers
	difSnapshotBuf  []*aclpb.RawChange
	tmpChangesBuf   []*Change
	newSnapshotsBuf []*Change
	notSeenIdxBuf   []int

	snapshotPath []string

	sync.RWMutex
}

func BuildObjectTree(treeStorage storage.TreeStorage, listener ObjectTreeUpdateListener, aclList list.ACLList) (ObjectTree, error) {
	treeBuilder := newTreeBuilder(treeStorage)
	validator := newTreeValidator()

	objTree := &objectTree{
		treeStorage:    treeStorage,
		tree:           nil,
		treeBuilder:    treeBuilder,
		validator:      validator,
		updateListener: listener,
		tmpChangesBuf:  make([]*Change, 0, 10),
		difSnapshotBuf: make([]*aclpb.RawChange, 0, 10),
		notSeenIdxBuf:  make([]int, 0, 10),
		kch:            newKeychain(),
	}
	err := objTree.rebuildFromStorage(aclList, nil)
	if err != nil {
		return nil, err
	}
	storageHeads, err := treeStorage.Heads()
	if err != nil {
		return nil, err
	}

	// comparing rebuilt heads with heads in storage
	// in theory it can happen that we didn't set heads because the process has crashed
	// therefore we want to set them later
	if !slice.UnsortedEquals(storageHeads, objTree.tree.Heads()) {
		log.With(zap.Strings("storage", storageHeads), zap.Strings("rebuilt", objTree.tree.Heads())).
			Errorf("the heads in storage and objTree are different")
		err = treeStorage.SetHeads(objTree.tree.Heads())
		if err != nil {
			return nil, err
		}
	}

	objTree.id, err = treeStorage.ID()
	if err != nil {
		return nil, err
	}
	objTree.header, err = treeStorage.Header()
	if err != nil {
		return nil, err
	}

	if listener != nil {
		listener.Rebuild(objTree)
	}

	return objTree, nil
}

func (ot *objectTree) rebuildFromStorage(aclList list.ACLList, newChanges []*Change) (err error) {
	ot.treeBuilder.Init(ot.kch)

	ot.tree, err = ot.treeBuilder.Build(newChanges)
	if err != nil {
		return
	}

	// during building the tree we may have marked some changes as possible roots,
	// but obviously they are not roots, because of the way how we construct the tree
	ot.tree.clearPossibleRoots()

	return ot.validator.ValidateTree(ot.tree, aclList)
}

func (ot *objectTree) ID() string {
	return ot.id
}

func (ot *objectTree) Header() *aclpb.Header {
	return ot.header
}

func (ot *objectTree) Storage() storage.TreeStorage {
	return ot.treeStorage
}

func (ot *objectTree) AddContent(ctx context.Context, aclList list.ACLList, content SignableChangeContent) (rawChange *aclpb.RawChange, err error) {
	defer func() {
		if ot.updateListener != nil {
			ot.updateListener.Update(ot)
		}
	}()
	state := aclList.ACLState() // special method for own keys
	aclChange := &aclpb.Change{
		TreeHeadIds:        ot.tree.Heads(),
		AclHeadId:          aclList.Head().Id,
		SnapshotBaseId:     ot.tree.RootId(),
		CurrentReadKeyHash: state.CurrentReadKeyHash(),
		Timestamp:          int64(time.Now().Nanosecond()),
		Identity:           content.Identity,
		IsSnapshot:         content.IsSnapshot,
	}

	marshalledData, err := content.Proto.Marshal()
	if err != nil {
		return nil, err
	}

	readKey, err := state.CurrentReadKey()
	if err != nil {
		return nil, err
	}

	encrypted, err := readKey.Encrypt(marshalledData)
	if err != nil {
		return nil, err
	}
	aclChange.ChangesData = encrypted

	fullMarshalledChange, err := proto.Marshal(aclChange)
	if err != nil {
		return nil, err
	}

	signature, err := content.Key.Sign(fullMarshalledChange)
	if err != nil {
		return nil, err
	}

	id, err := cid.NewCIDFromBytes(fullMarshalledChange)
	if err != nil {
		return nil, err
	}

	docChange := NewChange(id, aclChange, signature)
	docChange.ParsedModel = content

	if content.IsSnapshot {
		// clearing tree, because we already fixed everything in the last snapshot
		ot.tree = &Tree{}
	}
	err = ot.tree.AddMergedHead(docChange)
	if err != nil {
		panic(err)
	}
	rawChange = &aclpb.RawChange{
		Payload:   fullMarshalledChange,
		Signature: docChange.Signature(),
		Id:        docChange.Id,
	}

	err = ot.treeStorage.AddRawChange(rawChange)
	if err != nil {
		return
	}

	err = ot.treeStorage.SetHeads([]string{docChange.Id})
	return
}

func (ot *objectTree) AddRawChanges(ctx context.Context, aclList list.ACLList, rawChanges ...*aclpb.RawChange) (addResult AddResult, err error) {
	var mode Mode
	mode, addResult, err = ot.addRawChanges(ctx, aclList, rawChanges...)
	if err != nil {
		return
	}

	// reducing tree if we have new roots
	ot.tree.reduceTree()

	// adding to database all the added changes only after they are good
	for _, ch := range addResult.Added {
		err = ot.treeStorage.AddRawChange(ch)
		if err != nil {
			return
		}
	}

	// setting heads
	err = ot.treeStorage.SetHeads(ot.tree.Heads())
	if err != nil {
		return
	}

	if ot.updateListener == nil {
		return
	}

	switch mode {
	case Append:
		ot.updateListener.Update(ot)
	case Rebuild:
		ot.updateListener.Rebuild(ot)
	default:
		break
	}
	return
}

func (ot *objectTree) addRawChanges(ctx context.Context, aclList list.ACLList, rawChanges ...*aclpb.RawChange) (mode Mode, addResult AddResult, err error) {
	// resetting buffers
	ot.tmpChangesBuf = ot.tmpChangesBuf[:0]
	ot.notSeenIdxBuf = ot.notSeenIdxBuf[:0]
	ot.difSnapshotBuf = ot.difSnapshotBuf[:0]
	ot.newSnapshotsBuf = ot.newSnapshotsBuf[:0]

	// this will be returned to client so we shouldn't use buffer here
	prevHeadsCopy := make([]string, 0, len(ot.tree.Heads()))
	copy(prevHeadsCopy, ot.tree.Heads())

	// filtering changes, verifying and unmarshalling them
	for idx, ch := range rawChanges {
		if ot.HasChange(ch.Id) {
			continue
		}

		var change *Change
		change, err = newVerifiedChangeFromRaw(ch, ot.kch)
		if err != nil {
			return
		}

		if change.IsSnapshot {
			ot.newSnapshotsBuf = append(ot.newSnapshotsBuf, change)
		}
		ot.tmpChangesBuf = append(ot.tmpChangesBuf, change)
		ot.notSeenIdxBuf = append(ot.notSeenIdxBuf, idx)
	}

	// if no new changes, then returning
	if len(ot.notSeenIdxBuf) == 0 {
		addResult = AddResult{
			OldHeads: prevHeadsCopy,
			Heads:    prevHeadsCopy,
			Summary:  AddResultSummaryNothing,
		}
		return
	}

	headsCopy := func() []string {
		newHeads := make([]string, 0, len(ot.tree.Heads()))
		copy(newHeads, ot.tree.Heads())
		return newHeads
	}

	// returns changes that we added to the tree
	getAddedChanges := func() []*aclpb.RawChange {
		var added []*aclpb.RawChange
		for _, idx := range ot.notSeenIdxBuf {
			rawChange := rawChanges[idx]
			if _, exists := ot.tree.attached[rawChange.Id]; exists {
				added = append(added, rawChange)
			}
		}
		return added
	}

	rollback := func() {
		for _, ch := range ot.tmpChangesBuf {
			if _, exists := ot.tree.attached[ch.Id]; exists {
				delete(ot.tree.attached, ch.Id)
			} else if _, exists := ot.tree.unAttached[ch.Id]; exists {
				delete(ot.tree.unAttached, ch.Id)
			}
		}
	}

	// checks if we need to go to database
	isOldSnapshot := func(ch *Change) bool {
		if ch.SnapshotId == ot.tree.RootId() {
			return false
		}
		for _, sn := range ot.newSnapshotsBuf {
			// if change refers to newly received snapshot
			if ch.SnapshotId == sn.Id {
				return false
			}
		}
		return true
	}

	// checking if we have some changes with different snapshot and then rebuilding
	for _, ch := range ot.tmpChangesBuf {
		if isOldSnapshot(ch) {
			err = ot.rebuildFromStorage(aclList, ot.tmpChangesBuf)
			if err != nil {
				// rebuilding without new changes
				ot.rebuildFromStorage(aclList, nil)
				return
			}

			addResult = AddResult{
				OldHeads: prevHeadsCopy,
				Heads:    headsCopy(),
				Added:    getAddedChanges(),
				Summary:  AddResultSummaryRebuild,
			}
			return
		}
	}

	// normal mode of operation, where we don't need to rebuild from database
	mode = ot.tree.Add(ot.tmpChangesBuf...)
	switch mode {
	case Nothing:
		addResult = AddResult{
			OldHeads: prevHeadsCopy,
			Heads:    prevHeadsCopy,
			Summary:  AddResultSummaryNothing,
		}
		return

	default:
		// just rebuilding the state from start without reloading everything from tree storage
		// as an optimization we could've started from current heads, but I didn't implement that
		err = ot.validator.ValidateTree(ot.tree, aclList)
		if err != nil {
			rollback()
			err = ErrHasInvalidChanges
			return
		}

		addResult = AddResult{
			OldHeads: prevHeadsCopy,
			Heads:    headsCopy(),
			Added:    getAddedChanges(),
			Summary:  AddResultSummaryAppend,
		}
	}
	return
}

func (ot *objectTree) Iterate(f func(change *Change) bool) {
	ot.tree.Iterate(ot.tree.RootId(), f)
}

func (ot *objectTree) IterateFrom(s string, f func(change *Change) bool) {
	ot.tree.Iterate(s, f)
}

func (ot *objectTree) HasChange(s string) bool {
	_, attachedExists := ot.tree.attached[s]
	_, unattachedExists := ot.tree.unAttached[s]
	return attachedExists || unattachedExists
}

func (ot *objectTree) Heads() []string {
	return ot.tree.Heads()
}

func (ot *objectTree) Root() *Change {
	return ot.tree.Root()
}

func (ot *objectTree) Close() error {
	return nil
}

func (ot *objectTree) SnapshotPath() []string {
	if ot.snapshotPathIsActual() {
		return ot.snapshotPath
	}

	var path []string
	// TODO: think that the user may have not all of the snapshots locally
	currentSnapshotId := ot.tree.RootId()
	for currentSnapshotId != "" {
		sn, err := ot.treeBuilder.loadChange(currentSnapshotId)
		if err != nil {
			break
		}
		path = append(path, currentSnapshotId)
		currentSnapshotId = sn.SnapshotId
	}
	ot.snapshotPath = path

	return path
}

func (ot *objectTree) ChangesAfterCommonSnapshot(theirPath []string) ([]*aclpb.RawChange, error) {
	var (
		needFullDocument = len(theirPath) == 0
		ourPath          = ot.SnapshotPath()
		// by default returning everything we have
		commonSnapshot = ourPath[len(ourPath)-1]
		err            error
	)

	// if this is non-empty request
	if !needFullDocument {
		commonSnapshot, err = commonSnapshotForTwoPaths(ourPath, theirPath)
		if err != nil {
			return nil, err
		}
	}

	log.With(
		zap.Strings("heads", ot.tree.Heads()),
		zap.String("breakpoint", commonSnapshot),
		zap.String("id", ot.id)).
		Debug("getting all changes from common snapshot")

	if commonSnapshot == ot.tree.RootId() {
		return ot.getChangesFromTree()
	} else {
		return ot.getChangesFromDB(commonSnapshot, needFullDocument)
	}
}

func (ot *objectTree) getChangesFromTree() (rawChanges []*aclpb.RawChange, err error) {
	ot.tree.dfsPrev(ot.tree.HeadsChanges(), func(ch *Change) bool {
		var marshalled []byte
		marshalled, err = ch.Content.Marshal()
		if err != nil {
			return false
		}
		raw := &aclpb.RawChange{
			Payload:   marshalled,
			Signature: ch.Signature(),
			Id:        ch.Id,
		}
		rawChanges = append(rawChanges, raw)
		return true
	}, func(changes []*Change) {})

	return
}

func (ot *objectTree) getChangesFromDB(commonSnapshot string, needStartSnapshot bool) (rawChanges []*aclpb.RawChange, err error) {
	load := func(id string) (*Change, error) {
		raw, err := ot.treeStorage.GetRawChange(context.Background(), id)
		if err != nil {
			return nil, err
		}

		ch, err := NewChangeFromRaw(raw)
		if err != nil {
			return nil, err
		}

		rawChanges = append(rawChanges, raw)
		return ch, nil
	}

	_, err = ot.treeBuilder.dfs(ot.tree.Heads(), commonSnapshot, load)
	if err != nil {
		return
	}

	if needStartSnapshot {
		// adding snapshot to raw changes
		_, err = load(commonSnapshot)
	}

	return
}

func (ot *objectTree) snapshotPathIsActual() bool {
	return len(ot.snapshotPath) != 0 && ot.snapshotPath[len(ot.snapshotPath)-1] == ot.tree.RootId()
}

func (ot *objectTree) DebugDump() (string, error) {
	return ot.tree.Graph(NoOpDescriptionParser)
}

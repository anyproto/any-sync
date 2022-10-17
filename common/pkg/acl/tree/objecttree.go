//go:generate mockgen -destination mock_objecttree/mock_objecttree.go github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree ObjectTree
package tree

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/common"
	list2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/symmetric"
	"sync"
)

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

type AddResult struct {
	OldHeads []string
	Heads    []string
	Added    []*treechangeproto.RawTreeChangeWithId

	Mode Mode
}

type ChangeIterateFunc = func(change *Change) bool
type ChangeConvertFunc = func(decrypted []byte) (any, error)

type ObjectTree interface {
	RWLocker

	ID() string
	Header() *treechangeproto.RawTreeChangeWithId
	Heads() []string
	Root() *Change
	HasChanges(...string) bool
	DebugDump() (string, error)

	Iterate(convert ChangeConvertFunc, iterate ChangeIterateFunc) error
	IterateFrom(id string, convert ChangeConvertFunc, iterate ChangeIterateFunc) error

	SnapshotPath() []string
	ChangesAfterCommonSnapshot(snapshotPath, heads []string) ([]*treechangeproto.RawTreeChangeWithId, error)

	Storage() storage.TreeStorage

	AddContent(ctx context.Context, content SignableChangeContent) (AddResult, error)
	AddRawChanges(ctx context.Context, changes ...*treechangeproto.RawTreeChangeWithId) (AddResult, error)

	Close() error
}

type objectTree struct {
	treeStorage     storage.TreeStorage
	changeBuilder   ChangeBuilder
	validator       ObjectTreeValidator
	rawChangeLoader *rawChangeLoader
	treeBuilder     *treeBuilder
	aclList         list2.ACLList

	id   string
	root *treechangeproto.RawTreeChangeWithId
	tree *Tree

	keys map[uint64]*symmetric.Key

	// buffers
	difSnapshotBuf  []*treechangeproto.RawTreeChangeWithId
	newChangesBuf   []*Change
	newSnapshotsBuf []*Change
	notSeenIdxBuf   []int

	snapshotPath []string

	sync.RWMutex
}

type objectTreeDeps struct {
	changeBuilder   ChangeBuilder
	treeBuilder     *treeBuilder
	treeStorage     storage.TreeStorage
	validator       ObjectTreeValidator
	rawChangeLoader *rawChangeLoader
	aclList         list2.ACLList
}

func defaultObjectTreeDeps(
	rootChange *treechangeproto.RawTreeChangeWithId,
	treeStorage storage.TreeStorage,
	aclList list2.ACLList) objectTreeDeps {

	keychain := common.NewKeychain()
	changeBuilder := newChangeBuilder(keychain, rootChange)
	treeBuilder := newTreeBuilder(treeStorage, changeBuilder)
	return objectTreeDeps{
		changeBuilder:   changeBuilder,
		treeBuilder:     treeBuilder,
		treeStorage:     treeStorage,
		validator:       newTreeValidator(),
		rawChangeLoader: newRawChangeLoader(treeStorage, changeBuilder),
		aclList:         aclList,
	}
}

func (ot *objectTree) rebuildFromStorage(newChanges []*Change) (err error) {
	ot.treeBuilder.Reset()

	ot.tree, err = ot.treeBuilder.Build(newChanges)
	if err != nil {
		return
	}

	// during building the tree we may have marked some changes as possible roots,
	// but obviously they are not roots, because of the way how we construct the tree
	ot.tree.clearPossibleRoots()

	// it is a good question whether we need to validate everything
	// because maybe we can trust the stuff that is already in the storage
	return ot.validateTree(nil)
}

func (ot *objectTree) ID() string {
	return ot.id
}

func (ot *objectTree) Header() *treechangeproto.RawTreeChangeWithId {
	return ot.root
}

func (ot *objectTree) Storage() storage.TreeStorage {
	return ot.treeStorage
}

func (ot *objectTree) AddContent(ctx context.Context, content SignableChangeContent) (res AddResult, err error) {
	payload, err := ot.prepareBuilderContent(content)
	if err != nil {
		return
	}

	// saving old heads
	oldHeads := make([]string, 0, len(ot.tree.Heads()))
	oldHeads = append(oldHeads, ot.tree.Heads()...)

	objChange, rawChange, err := ot.changeBuilder.BuildContent(payload)
	if content.IsSnapshot {
		// clearing tree, because we already saved everything in the last snapshot
		ot.tree = &Tree{}
	}
	err = ot.tree.AddMergedHead(objChange)
	if err != nil {
		panic(err)
	}

	err = ot.treeStorage.AddRawChange(rawChange)
	if err != nil {
		return
	}

	err = ot.treeStorage.SetHeads([]string{objChange.Id})
	if err != nil {
		return
	}

	res = AddResult{
		OldHeads: oldHeads,
		Heads:    []string{objChange.Id},
		Added:    []*treechangeproto.RawTreeChangeWithId{rawChange},
		Mode:     Append,
	}
	return
}

func (ot *objectTree) prepareBuilderContent(content SignableChangeContent) (cnt BuilderContent, err error) {
	ot.aclList.RLock()
	defer ot.aclList.RUnlock()

	state := ot.aclList.ACLState() // special method for own keys
	readKey, err := state.CurrentReadKey()
	if err != nil {
		return
	}
	cnt = BuilderContent{
		TreeHeadIds:        ot.tree.Heads(),
		AclHeadId:          ot.aclList.Head().Id,
		SnapshotBaseId:     ot.tree.RootId(),
		CurrentReadKeyHash: state.CurrentReadKeyHash(),
		Identity:           content.Identity,
		IsSnapshot:         content.IsSnapshot,
		SigningKey:         content.Key,
		ReadKey:            readKey,
		Content:            content.Data,
	}
	return
}

func (ot *objectTree) AddRawChanges(ctx context.Context, rawChanges ...*treechangeproto.RawTreeChangeWithId) (addResult AddResult, err error) {
	addResult, err = ot.addRawChanges(ctx, rawChanges...)
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
	return
}

func (ot *objectTree) addRawChanges(ctx context.Context, rawChanges ...*treechangeproto.RawTreeChangeWithId) (addResult AddResult, err error) {
	// resetting buffers
	ot.newChangesBuf = ot.newChangesBuf[:0]
	ot.notSeenIdxBuf = ot.notSeenIdxBuf[:0]
	ot.difSnapshotBuf = ot.difSnapshotBuf[:0]
	ot.newSnapshotsBuf = ot.newSnapshotsBuf[:0]

	headsCopy := func() []string {
		newHeads := make([]string, 0, len(ot.tree.Heads()))
		newHeads = append(newHeads, ot.tree.Heads()...)
		return newHeads
	}

	// this will be returned to client, so we shouldn't use buffer here
	prevHeadsCopy := headsCopy()

	// filtering changes, verifying and unmarshalling them
	for idx, ch := range rawChanges {
		// not unmarshalling the changes if they were already added either as unattached or attached
		if _, exists := ot.tree.attached[ch.Id]; exists {
			continue
		}

		var change *Change
		if unAttached, exists := ot.tree.unAttached[ch.Id]; exists {
			change = unAttached
		} else {
			change, err = ot.changeBuilder.ConvertFromRaw(ch, true)
			if err != nil {
				return
			}
		}

		if change.IsSnapshot {
			ot.newSnapshotsBuf = append(ot.newSnapshotsBuf, change)
		}
		ot.newChangesBuf = append(ot.newChangesBuf, change)
		ot.notSeenIdxBuf = append(ot.notSeenIdxBuf, idx)
	}

	// if no new changes, then returning
	if len(ot.notSeenIdxBuf) == 0 {
		addResult = AddResult{
			OldHeads: prevHeadsCopy,
			Heads:    prevHeadsCopy,
			Mode:     Nothing,
		}
		return
	}

	rollback := func(changes []*Change) {
		for _, ch := range changes {
			if _, exists := ot.tree.attached[ch.Id]; exists {
				delete(ot.tree.attached, ch.Id)
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

	shouldRebuildFromStorage := false
	// checking if we have some changes with different snapshot and then rebuilding
	for idx, ch := range ot.newChangesBuf {
		if isOldSnapshot(ch) {
			var exists bool
			// checking if it exists in the storage, if yes, then at some point it was added to the tree
			// thus we don't need to look at this change
			exists, err = ot.treeStorage.HasChange(ctx, ch.Id)
			if err != nil {
				return
			}
			if exists {
				// marking as nil to delete after
				ot.newChangesBuf[idx] = nil
				continue
			}
			// we haven't seen the change, and it refers to old snapshot, so we should rebuild
			shouldRebuildFromStorage = true
		}
	}
	// discarding all previously seen changes
	ot.newChangesBuf = discardFromSlice(ot.newChangesBuf, func(ch *Change) bool { return ch == nil })

	if shouldRebuildFromStorage {
		err = ot.rebuildFromStorage(ot.newChangesBuf)
		if err != nil {
			// rebuilding without new changes
			ot.rebuildFromStorage(nil)
			return
		}
		addResult, err = ot.createAddResult(prevHeadsCopy, Rebuild, nil, rawChanges)
		if err != nil {
			// that means that some unattached changes were somehow corrupted in memory
			// this shouldn't happen but if that happens, then rebuilding from storage
			ot.rebuildFromStorage(nil)
			return
		}
		return
	}

	// normal mode of operation, where we don't need to rebuild from database
	mode, treeChangesAdded := ot.tree.Add(ot.newChangesBuf...)
	switch mode {
	case Nothing:
		addResult = AddResult{
			OldHeads: prevHeadsCopy,
			Heads:    prevHeadsCopy,
			Mode:     mode,
		}
		return

	default:
		// we need to validate only newly added changes
		err = ot.validateTree(treeChangesAdded)
		if err != nil {
			rollback(treeChangesAdded)
			err = ErrHasInvalidChanges
			return
		}
		addResult, err = ot.createAddResult(prevHeadsCopy, mode, treeChangesAdded, rawChanges)
		if err != nil {
			// that means that some unattached changes were somehow corrupted in memory
			// this shouldn't happen but if that happens, then rebuilding from storage
			ot.rebuildFromStorage(nil)
			return
		}
		return
	}
	return
}

func (ot *objectTree) createAddResult(oldHeads []string, mode Mode, treeChangesAdded []*Change, rawChanges []*treechangeproto.RawTreeChangeWithId) (addResult AddResult, err error) {
	headsCopy := func() []string {
		newHeads := make([]string, 0, len(ot.tree.Heads()))
		newHeads = append(newHeads, ot.tree.Heads()...)
		return newHeads
	}

	// returns changes that we added to the tree as attached this round
	// they can include not only the changes that were added now,
	// but also the changes that were previously in the tree
	getAddedChanges := func(toConvert []*Change) (added []*treechangeproto.RawTreeChangeWithId, err error) {
		alreadyConverted := make(map[*Change]struct{})

		// first we see if we have already unmarshalled those changes
		for _, idx := range ot.notSeenIdxBuf {
			rawChange := rawChanges[idx]
			if ch, exists := ot.tree.attached[rawChange.Id]; exists {
				if len(toConvert) != 0 {
					alreadyConverted[ch] = struct{}{}
				}
				added = append(added, rawChange)
			}
		}
		// this will happen in case we called rebuild from storage
		// or if all the changes that we added were contained in current add request
		// (this what would happen in most cases)
		if len(toConvert) == 0 || len(added) == len(toConvert) {
			return
		}

		// but in some cases it may happen that the changes that were added this round
		// were contained in unattached from previous requests
		for _, ch := range toConvert {
			// if we got some changes that we need to convert to raw
			if _, exists := alreadyConverted[ch]; !exists {
				var raw *treechangeproto.RawTreeChangeWithId
				raw, err = ot.changeBuilder.BuildRaw(ch)
				if err != nil {
					return
				}
				added = append(added, raw)
			}
		}
		return
	}

	var added []*treechangeproto.RawTreeChangeWithId
	added, err = getAddedChanges(treeChangesAdded)
	if err != nil {
		return
	}
	addResult = AddResult{
		OldHeads: oldHeads,
		Heads:    headsCopy(),
		Added:    added,
		Mode:     mode,
	}
	return
}

func (ot *objectTree) Iterate(convert ChangeConvertFunc, iterate ChangeIterateFunc) (err error) {
	return ot.IterateFrom(ot.tree.RootId(), convert, iterate)
}

func (ot *objectTree) IterateFrom(id string, convert ChangeConvertFunc, iterate ChangeIterateFunc) (err error) {
	if convert == nil {
		ot.tree.Iterate(id, iterate)
		return
	}

	ot.tree.Iterate(ot.tree.RootId(), func(c *Change) (isContinue bool) {
		var model any
		if c.Model != nil {
			return iterate(c)
		}
		// if this is a root change
		if c.Id == ot.id {
			return iterate(c)
		}
		readKey, exists := ot.keys[c.ReadKeyHash]
		if !exists {
			err = list2.ErrNoReadKey
			return false
		}

		var decrypted []byte
		decrypted, err = readKey.Decrypt(c.Data)
		if err != nil {
			return false
		}

		model, err = convert(decrypted)
		if err != nil {
			return false
		}

		c.Model = model
		return iterate(c)
	})
	return
}

func (ot *objectTree) HasChanges(chs ...string) bool {
	hasChange := func(s string) bool {
		_, attachedExists := ot.tree.attached[s]
		if attachedExists {
			return attachedExists
		}

		has, err := ot.treeStorage.HasChange(context.Background(), s)
		if err != nil {
			return false
		}

		return has
	}

	for _, ch := range chs {
		if !hasChange(ch) {
			return false
		}
	}

	return true
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
	// TODO: Add error as return parameter
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

func (ot *objectTree) ChangesAfterCommonSnapshot(theirPath, theirHeads []string) ([]*treechangeproto.RawTreeChangeWithId, error) {
	var (
		needFullDocument = len(theirPath) == 0
		ourPath          = ot.SnapshotPath()
		// by default returning everything we have from start
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

	if commonSnapshot == ot.tree.RootId() {
		return ot.getChangesFromTree(theirHeads)
	} else {
		return ot.getChangesFromDB(commonSnapshot, theirHeads)
	}
}

func (ot *objectTree) getChangesFromTree(theirHeads []string) (rawChanges []*treechangeproto.RawTreeChangeWithId, err error) {
	return ot.rawChangeLoader.LoadFromTree(ot.tree, theirHeads)
}

func (ot *objectTree) getChangesFromDB(commonSnapshot string, theirHeads []string) (rawChanges []*treechangeproto.RawTreeChangeWithId, err error) {
	return ot.rawChangeLoader.LoadFromStorage(commonSnapshot, ot.tree.headIds, theirHeads)
}

func (ot *objectTree) snapshotPathIsActual() bool {
	return len(ot.snapshotPath) != 0 && ot.snapshotPath[0] == ot.tree.RootId()
}

func (ot *objectTree) validateTree(newChanges []*Change) error {
	ot.aclList.RLock()
	defer ot.aclList.RUnlock()
	state := ot.aclList.ACLState()

	// just not to take lock many times, updating the key map from aclList
	if len(ot.keys) != len(state.UserReadKeys()) {
		for key, value := range state.UserReadKeys() {
			ot.keys[key] = value
		}
	}
	if len(newChanges) == 0 {
		return ot.validator.ValidateFullTree(ot.tree, ot.aclList)
	}

	return ot.validator.ValidateNewChanges(ot.tree, ot.aclList, newChanges)
}

func (ot *objectTree) DebugDump() (string, error) {
	return ot.tree.Graph(NoOpDescriptionParser)
}

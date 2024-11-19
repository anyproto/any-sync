//go:generate mockgen -destination mock_objecttree/mock_objecttree.go github.com/anyproto/any-sync/commonspace/object/tree/objecttree ObjectTree
package objecttree

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/anyproto/any-sync/util/debug"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/anyproto/any-sync/util/slice"
)

var (
	ErrHasInvalidChanges = errors.New("the change is invalid")
	ErrNoCommonSnapshot  = errors.New("trees doesn't have a common snapshot")
	ErrNoChangeInTree    = errors.New("no such change in tree")
	ErrMissingKey        = errors.New("missing current read key")
	ErrDerived           = errors.New("expect >= 2 changes in derived tree")
	ErrDeleted           = errors.New("object tree is deleted")
	ErrNoAclHead         = errors.New("no acl head")
)

type (
	Updater         = func(tree ObjectTree, md Mode) error
	ChangeValidator = func(change *treechangeproto.RawTreeChangeWithId) error
)

type AddResultSummary int

type AddResult struct {
	OldHeads []string
	Heads    []string
	Added    []StorageChange

	Mode Mode
}

type RawChangesPayload struct {
	NewHeads     []string
	RawChanges   []*treechangeproto.RawTreeChangeWithId
	SnapshotPath []string
}

type ChangeIterateFunc = func(change *Change) bool
type ChangeConvertFunc = func(change *Change, decrypted []byte) (any, error)

type TryLocker interface {
	sync.Locker
	TryLock() bool
}

type ReadableObjectTree interface {
	TryLocker

	Id() string
	Header() *treechangeproto.RawTreeChangeWithId
	UnmarshalledHeader() *Change
	ChangeInfo() *treechangeproto.TreeChangeInfo
	Heads() []string
	Root() *Change
	Len() int
	IsDerived() bool

	AclList() list.AclList

	HasChanges(...string) bool
	GetChange(string) (*Change, error)

	Debug(parser DescriptionParser) (DebugInfo, error)
	IterateRoot(convert ChangeConvertFunc, iterate ChangeIterateFunc) error
	IterateFrom(id string, convert ChangeConvertFunc, iterate ChangeIterateFunc) error
}

type ObjectTree interface {
	ReadableObjectTree

	SnapshotPath() []string
	ChangesAfterCommonSnapshotLoader(snapshotPath, heads []string) (LoadIterator, error)

	Storage() treestorage.TreeStorage

	AddContent(ctx context.Context, content SignableChangeContent) (AddResult, error)
	AddContentWithValidator(ctx context.Context, content SignableChangeContent, validate ChangeValidator) (AddResult, error)
	AddRawChanges(ctx context.Context, changes RawChangesPayload) (AddResult, error)
	AddRawChangesWithUpdater(ctx context.Context, changes RawChangesPayload, updater Updater) (AddResult, error)

	UnpackChange(raw *treechangeproto.RawTreeChangeWithId) (data []byte, err error)
	PrepareChange(content SignableChangeContent) (res *treechangeproto.RawTreeChangeWithId, err error)

	Delete() error
	Close() error
	SetFlusher(flusher Flusher)
	TryClose(objectTTL time.Duration) (bool, error)
}

type objectTree struct {
	treeStorage   treestorage.TreeStorage
	storage       Storage
	changeBuilder ChangeBuilder
	validator     ObjectTreeValidator
	treeBuilder   *treeBuilder
	aclList       list.AclList
	flusher       Flusher

	id      string
	rawRoot *treechangeproto.RawTreeChangeWithId
	root    *Change
	tree    *Tree

	keys           map[string]crypto.SymKey
	currentReadKey crypto.SymKey
	isDeleted      bool

	// buffers
	difSnapshotBuf  []*treechangeproto.RawTreeChangeWithId
	newChangesBuf   []*Change
	newSnapshotsBuf []*Change
	notSeenIdxBuf   []int

	snapshotPath []string

	sync.Mutex
}

func (ot *objectTree) rebuildFromStorage(theirHeads, theirSnapshotPath []string, newChanges []*Change) (err error) {
	var (
		ourPath []string
		oldTree = ot.tree
	)
	if ot.tree != nil {
		ourPath = ot.SnapshotPath()
	}
	ot.tree, err = ot.treeBuilder.Build(treeBuilderOpts{
		full:              false,
		theirHeads:        theirHeads,
		ourHeads:          ot.tree.Heads(),
		ourSnapshotPath:   ourPath,
		theirSnapshotPath: theirSnapshotPath,
		newChanges:        newChanges,
	})
	if err != nil {
		ot.tree = oldTree
		return
	}

	// in case there are new heads
	if theirHeads != nil && oldTree != nil {
		// checking that old root is still in tree
		rootCh, rootExists := ot.tree.attached[oldTree.RootId()]

		// checking the case where theirHeads were actually below prevHeads
		// so if we did load some extra data in the tree, let's reduce it to old root
		if slice.UnsortedEquals(oldTree.headIds, ot.tree.headIds) && rootExists && ot.tree.RootId() != oldTree.RootId() {
			ot.tree.makeRootAndRemove(rootCh)
		}
	}

	// during building the tree we may have marked some changes as possible roots,
	// but obviously they are not roots, because of the way how we construct the tree
	ot.tree.clearPossibleRoots()

	// it is a good question whether we need to validate everything
	// because maybe we can trust the stuff that is already in the storage
	return ot.validateTree(nil)
}

func (ot *objectTree) Id() string {
	return ot.id
}

func (ot *objectTree) IsDerived() bool {
	return ot.root.IsDerived
}

func (ot *objectTree) Len() int {
	return ot.tree.Len()
}

func (ot *objectTree) AclList() list.AclList {
	return ot.aclList
}

func (ot *objectTree) Header() *treechangeproto.RawTreeChangeWithId {
	return ot.rawRoot
}

func (ot *objectTree) SetFlusher(flusher Flusher) {
	ot.flusher = flusher
}

func (ot *objectTree) UnmarshalledHeader() *Change {
	return ot.root
}

func (ot *objectTree) ChangeInfo() *treechangeproto.TreeChangeInfo {
	return ot.root.Model.(*treechangeproto.TreeChangeInfo)
}

func (ot *objectTree) Storage() treestorage.TreeStorage {
	return ot.treeStorage
}

func (ot *objectTree) GetChange(id string) (*Change, error) {
	if ot.isDeleted {
		return nil, ErrDeleted
	}
	if ch, ok := ot.tree.attached[id]; ok {
		return ch, nil
	}
	return nil, ErrNoChangeInTree
}

func (ot *objectTree) logUseWhenUnlocked() {
	// this is needed to check when we use the tree not under the lock
	if ot.TryLock() {
		log.With(zap.String("treeId", ot.id), zap.String("stack", debug.StackCompact(true))).Error("use tree when unlocked")
		ot.Unlock()
	}
}

func (ot *objectTree) AddContent(ctx context.Context, content SignableChangeContent) (res AddResult, err error) {
	return ot.AddContentWithValidator(ctx, content, nil)
}

func (ot *objectTree) AddContentWithValidator(ctx context.Context, content SignableChangeContent, validator func(change *treechangeproto.RawTreeChangeWithId) error) (res AddResult, err error) {
	if ot.isDeleted {
		err = ErrDeleted
		return
	}
	ot.logUseWhenUnlocked()
	payload, err := ot.prepareBuilderContent(content)
	if err != nil {
		return
	}

	// saving old heads
	oldHeads := make([]string, 0, len(ot.tree.Heads()))
	oldHeads = append(oldHeads, ot.tree.Heads()...)

	objChange, rawChange, err := ot.changeBuilder.Build(payload)
	if err != nil {
		return
	}
	objChange.OrderId = lexId.Next(ot.tree.attached[ot.tree.lastIteratedHeadId].OrderId)
	if content.IsSnapshot {
		objChange.SnapshotCounter = ot.tree.root.SnapshotCounter + 1
		// clearing tree, because we already saved everything in the last snapshot
		ot.tree = &Tree{}
	}

	if validator != nil {
		err = validator(rawChange)
		if err != nil {
			return
		}
	}

	err = ot.tree.AddMergedHead(objChange)
	if err != nil {
		panic(err)
	}
	storageChange := StorageChange{
		RawChange:       rawChange.RawChange,
		PrevIds:         objChange.PreviousIds,
		Id:              objChange.Id,
		SnapshotCounter: objChange.SnapshotCounter,
		SnapshotId:      objChange.SnapshotId,
		OrderId:         objChange.OrderId,
		ChangeSize:      len(rawChange.RawChange),
	}
	err = ot.storage.AddAll(ctx, []StorageChange{storageChange}, ot.Heads(), ot.tree.root.Id)
	if err != nil {
		return
	}

	mode := Append
	if content.IsSnapshot {
		mode = Rebuild
	}

	res = AddResult{
		OldHeads: oldHeads,
		Heads:    []string{objChange.Id},
		Added:    []StorageChange{storageChange},
		Mode:     mode,
	}
	log.With("treeId", ot.id).With("head", objChange.Id).
		Debug("finished adding content")
	return
}

func (ot *objectTree) UnpackChange(raw *treechangeproto.RawTreeChangeWithId) (data []byte, err error) {
	if ot.isDeleted {
		err = ErrDeleted
		return
	}
	unmarshalled, err := ot.changeBuilder.Unmarshall(raw, true)
	if err != nil {
		return
	}
	data = unmarshalled.Data
	return
}

func (ot *objectTree) PrepareChange(content SignableChangeContent) (res *treechangeproto.RawTreeChangeWithId, err error) {
	if ot.isDeleted {
		err = ErrDeleted
		return
	}
	payload, err := ot.prepareBuilderContent(content)
	if err != nil {
		return
	}
	_, res, err = ot.changeBuilder.Build(payload)
	return
}

func (ot *objectTree) prepareBuilderContent(content SignableChangeContent) (cnt BuilderContent, err error) {
	ot.aclList.RLock()
	defer ot.aclList.RUnlock()

	var (
		state     = ot.aclList.AclState() // special method for own keys
		readKey   crypto.SymKey
		pubKey    = content.Key.GetPublic()
		readKeyId string
	)
	if !state.Permissions(pubKey).CanWrite() {
		err = list.ErrInsufficientPermissions
		return
	}

	if content.IsEncrypted {
		err = ot.readKeysFromAclState(state)
		if err != nil {
			return
		}
		readKeyId = state.CurrentReadKeyId()
		if ot.currentReadKey == nil {
			err = ErrMissingKey
			return
		}
		readKey = ot.currentReadKey
	}
	timestamp := content.Timestamp
	if timestamp <= 0 {
		timestamp = time.Now().Unix()
	}
	cnt = BuilderContent{
		TreeHeadIds:    ot.tree.Heads(),
		AclHeadId:      ot.aclList.Head().Id,
		SnapshotBaseId: ot.tree.RootId(),
		ReadKeyId:      readKeyId,
		IsSnapshot:     content.IsSnapshot,
		PrivKey:        content.Key,
		ReadKey:        readKey,
		Content:        content.Data,
		DataType:       content.DataType,
		Timestamp:      timestamp,
	}
	return
}

func (ot *objectTree) AddRawChangesWithUpdater(ctx context.Context, changes RawChangesPayload, updater Updater) (addResult AddResult, err error) {
	if ot.isDeleted {
		err = ErrDeleted
		return
	}
	ot.logUseWhenUnlocked()
	lastHeadId := ot.tree.lastIteratedHeadId
	addResult, err = ot.addChangesToTree(ctx, changes)
	if err != nil {
		return
	}

	// reducing tree if we have new roots
	err = ot.flusher.FlushAfterBuild(ot)
	if err != nil {
		return
	}

	// that means that we removed the ids while reducing
	if _, exists := ot.tree.attached[lastHeadId]; !exists {
		addResult.Mode = Rebuild
	}

	rollback := func() {
		rebuildErr := ot.rebuildFromStorage(nil, nil, nil)
		if rebuildErr != nil {
			log.Error("failed to rebuild after adding to storage", zap.Strings("heads", ot.Heads()), zap.Error(rebuildErr))
		}
	}

	if updater != nil {
		err = updater(ot, addResult.Mode)
		if err != nil {
			rollback()
			return
		}
	}

	err = ot.storage.AddAll(ctx, addResult.Added, addResult.Heads, ot.tree.RootId())
	if err != nil {
		rollback()
		return
	}
	ot.flusher.Flush(ot)
	return
}

func (ot *objectTree) AddRawChanges(ctx context.Context, changesPayload RawChangesPayload) (addResult AddResult, err error) {
	return ot.AddRawChangesWithUpdater(ctx, changesPayload, nil)
}

func (ot *objectTree) addChangesToTree(ctx context.Context, changesPayload RawChangesPayload) (addResult AddResult, err error) {
	// resetting buffers
	ot.newChangesBuf = ot.newChangesBuf[:0]
	ot.notSeenIdxBuf = ot.notSeenIdxBuf[:0]
	ot.difSnapshotBuf = ot.difSnapshotBuf[:0]
	ot.newSnapshotsBuf = ot.newSnapshotsBuf[:0]

	headsCopy := func(heads []string) []string {
		newHeads := make([]string, 0, len(ot.tree.Heads()))
		newHeads = append(newHeads, heads...)
		return newHeads
	}

	var (
		// this will be returned to client, so we shouldn't use buffer here
		prevHeadsCopy  = headsCopy(ot.tree.Heads())
		lastIteratedId = ot.tree.lastIteratedHeadId
	)

	// filtering changes, verifying and unmarshalling them
	for idx, ch := range changesPayload.RawChanges {
		// not unmarshalling the changes if they were already added either as unattached or attached
		if _, exists := ot.tree.attached[ch.Id]; exists {
			continue
		}

		var change *Change
		if unAttached, exists := ot.tree.unAttached[ch.Id]; exists {
			change = unAttached
		} else {
			change, err = ot.changeBuilder.Unmarshall(ch, true)
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

	var (
		filteredHeads = false
		headsToUse    = changesPayload.NewHeads
	)
	// if our validator provides filtering mechanism then we use it
	filteredHeads, ot.newChangesBuf, ot.newSnapshotsBuf, ot.notSeenIdxBuf = ot.validator.FilterChanges(ot.aclList, ot.newChangesBuf, ot.newSnapshotsBuf, ot.notSeenIdxBuf)
	if filteredHeads {
		// if we filtered some of the heads, then we don't know which heads to use
		headsToUse = []string{}
	}
	rollback := func(changes []*Change) {
		var visited []*Change
		for _, ch := range changes {
			if ex, exists := ot.tree.attached[ch.Id]; exists {
				ex.visited = true
				visited = append(visited, ex)
				delete(ot.tree.attached, ch.Id)
			}
		}
		for _, ch := range ot.tree.attached {
			// deleting all visited changes from next
			ch.Next = slice.DiscardFromSlice(ch.Next, func(change *Change) bool {
				return change.visited
			})
		}
		// doing this just in case
		for _, ch := range visited {
			ch.visited = false
		}
		ot.tree.headIds = headsCopy(prevHeadsCopy)
		ot.tree.lastIteratedHeadId = lastIteratedId
	}

	// checks if we need to go to database
	snapshotNotInTree := func(ch *Change) (bool, error) {
		if ch.SnapshotId == ot.tree.RootId() {
			return false, nil
		}
		if oldSn, ok := ot.tree.attached[ch.SnapshotId]; ok {
			if !oldSn.IsSnapshot {
				return false, ErrHasInvalidChanges
			}
		}
		for _, sn := range ot.newSnapshotsBuf {
			// if change refers to newly received snapshot
			if ch.SnapshotId == sn.Id {
				return false, nil
			}
		}
		return true, nil
	}

	shouldRebuildFromStorage := false
	// checking if we have some changes with different snapshot and then rebuilding
	for _, ch := range ot.newChangesBuf {
		var notInTree bool
		notInTree, err = snapshotNotInTree(ch)
		if err != nil {
			return
		}
		if notInTree {
			shouldRebuildFromStorage = true
			break
		}
	}
	log := log.With(zap.String("treeId", ot.id))
	if shouldRebuildFromStorage {
		err = ot.rebuildFromStorage(headsToUse, changesPayload.SnapshotPath, ot.newChangesBuf)
		if err != nil {
			log.Error("failed to rebuild with new heads", zap.Strings("headsToUse", headsToUse), zap.Error(err))
			// rebuilding without new changes
			rebuildErr := ot.rebuildFromStorage(nil, nil, nil)
			if rebuildErr != nil {
				log.Error("failed to rebuild from storage", zap.Strings("heads", ot.Heads()), zap.Error(rebuildErr))
			}
			return
		}
		addResult, err = ot.createAddResult(prevHeadsCopy, Rebuild, changesPayload.RawChanges)
		if err != nil {
			log.Error("failed to create add result", zap.Strings("headsToUse", headsToUse), zap.Error(err))
			// that means that some unattached changes were somehow corrupted in memory
			// this shouldn't happen but if that happens, then rebuilding from storage
			rebuildErr := ot.rebuildFromStorage(nil, nil, nil)
			if rebuildErr != nil {
				log.Error("failed to rebuild after add result", zap.Strings("heads", ot.Heads()), zap.Error(rebuildErr))
			}
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
			err = fmt.Errorf("%w: %w", ErrHasInvalidChanges, err)
			return
		}
		addResult, err = ot.createAddResult(prevHeadsCopy, mode, changesPayload.RawChanges)
		if err != nil {
			// that means that some unattached changes were somehow corrupted in memory
			// this shouldn't happen but if that happens, then rebuilding from storage
			rollback(treeChangesAdded)
			rebuildErr := ot.rebuildFromStorage(nil, nil, nil)
			if rebuildErr != nil {
				log.Error("failed to rebuild after add result (add to tree)", zap.Strings("heads", ot.Heads()), zap.Error(rebuildErr))
			}
			return
		}
		return
	}
}

func (ot *objectTree) createAddResult(oldHeads []string, mode Mode, rawChanges []*treechangeproto.RawTreeChangeWithId) (addResult AddResult, err error) {
	headsCopy := func() []string {
		newHeads := make([]string, 0, len(ot.tree.Heads()))
		newHeads = append(newHeads, ot.tree.Heads()...)
		return newHeads
	}

	// returns changes that we added to the tree as attached this round
	// they can include not only the changes that were added now,
	// but also the changes that were previously in the tree
	getAddedChanges := func() (added []StorageChange, err error) {
		for _, idx := range ot.notSeenIdxBuf {
			rawChange := rawChanges[idx]
			if ch, exists := ot.tree.attached[rawChange.Id]; exists {
				ot.flusher.MarkNewChange(ch)
				added = append(added, StorageChange{
					RawChange:       rawChange.RawChange,
					PrevIds:         ch.PreviousIds,
					Id:              ch.Id,
					SnapshotCounter: ch.SnapshotCounter,
					SnapshotId:      ch.SnapshotId,
					OrderId:         ch.OrderId,
					ChangeSize:      len(rawChange.RawChange),
				})
			}
		}
		return
	}

	var added []StorageChange
	added, err = getAddedChanges()
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

func (ot *objectTree) IterateRoot(convert ChangeConvertFunc, iterate ChangeIterateFunc) (err error) {
	return ot.IterateFrom(ot.tree.RootId(), convert, iterate)
}

func (ot *objectTree) IterateFrom(id string, convert ChangeConvertFunc, iterate ChangeIterateFunc) (err error) {
	if ot.isDeleted {
		err = ErrDeleted
		return
	}
	if convert == nil {
		ot.tree.IterateSkip(id, iterate)
		return
	}
	decrypt := func(c *Change) (decrypted []byte, err error) {
		// the change is not encrypted
		if c.ReadKeyId == "" {
			decrypted = c.Data
			return
		}
		readKey, exists := ot.keys[c.ReadKeyId]
		if !exists {
			err = list.ErrNoReadKey
			return
		}

		decrypted, err = readKey.Decrypt(c.Data)
		return
	}

	ot.tree.IterateSkip(id, func(c *Change) (isContinue bool) {
		var model any
		// if already saved as a model
		if c.Model != nil {
			return iterate(c)
		}
		// if this is a root change
		if c.Id == ot.id {
			return iterate(c)
		}

		var decrypted []byte
		decrypted, err = decrypt(c)
		if err != nil {
			return false
		}

		model, err = convert(c, decrypted)
		if err != nil {
			return false
		}

		c.Model = model
		// delete data because it just wastes memory
		c.Data = nil
		return iterate(c)
	})
	return
}

func (ot *objectTree) HasChanges(chs ...string) bool {
	if ot.isDeleted {
		return false
	}
	for _, ch := range chs {
		if _, attachedExists := ot.tree.attached[ch]; !attachedExists {
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

func (ot *objectTree) TryClose(objectTTL time.Duration) (bool, error) {
	return true, ot.Close()
}

func (ot *objectTree) Close() error {
	return nil
}

func (ot *objectTree) Delete() error {
	if ot.isDeleted {
		return nil
	}
	ot.isDeleted = true
	return ot.treeStorage.Delete()
}

func (ot *objectTree) SnapshotPath() []string {
	if ot.isDeleted {
		return nil
	}
	// TODO: Add error as return parameter
	if ot.snapshotPathIsActual() {
		return ot.snapshotPath
	}

	var path []string
	// TODO: think that the user may have not all of the snapshots locally
	currentSnapshotId := ot.tree.RootId()
	for currentSnapshotId != "" {
		sn, err := ot.storage.Get(context.Background(), currentSnapshotId)
		if err != nil {
			break
		}
		path = append(path, currentSnapshotId)
		currentSnapshotId = sn.SnapshotId
	}
	ot.snapshotPath = path

	return path
}

func (ot *objectTree) ChangesAfterCommonSnapshotLoader(theirPath, theirHeads []string) (LoadIterator, error) {
	if ot.isDeleted {
		return nil, ErrDeleted
	}
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

	iter := newLoadIterator(ot.rawRoot, ourPath, ot.storage, ot.changeBuilder)
	err = iter.load(commonSnapshot, ot.tree.headIds, theirHeads)
	if err != nil {
		return nil, err
	}
	return iter, nil
}

func (ot *objectTree) snapshotPathIsActual() bool {
	return len(ot.snapshotPath) != 0 && ot.snapshotPath[0] == ot.tree.RootId()
}

func (ot *objectTree) validateTree(newChanges []*Change) error {
	ot.aclList.RLock()
	defer ot.aclList.RUnlock()
	state := ot.aclList.AclState()

	err := ot.readKeysFromAclState(state)
	if err != nil {
		return err
	}
	if len(newChanges) == 0 {
		return ot.validator.ValidateFullTree(ot.tree, ot.aclList)
	}

	return ot.validator.ValidateNewChanges(ot.tree, ot.aclList, newChanges)
}

func (ot *objectTree) readKeysFromAclState(state *list.AclState) (err error) {
	// just not to take lock many times, updating the key map from aclList
	if len(ot.keys) == len(state.Keys()) {
		return nil
	}
	// if we can't read the keys anyway
	if state.AccountKey() == nil || !state.HadReadPermissions(state.AccountKey().GetPublic()) {
		return nil
	}
	for key, value := range state.Keys() {
		if _, exists := ot.keys[key]; exists {
			continue
		}
		if value.ReadKey == nil {
			continue
		}
		treeKey, err := deriveTreeKey(value.ReadKey, ot.id)
		if err != nil {
			return err
		}
		ot.keys[key] = treeKey
	}
	curKey, err := state.CurrentReadKey()
	if err != nil {
		return err
	}
	if curKey == nil {
		return nil
	}
	ot.currentReadKey, err = deriveTreeKey(curKey, ot.id)
	return err
}

func (ot *objectTree) Debug(parser DescriptionParser) (DebugInfo, error) {
	return objectTreeDebug{}.debugInfo(ot, parser)
}

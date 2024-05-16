//go:generate mockgen -destination mock_objecttree/mock_objecttree.go github.com/anyproto/any-sync/commonspace/object/tree/objecttree ObjectTree
package objecttree

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/anyproto/any-sync/util/slice"
)

type RWLocker interface {
	sync.Locker
	RLock()
	RUnlock()
	TryRLock() bool
	TryLock() bool
}

var (
	ErrHasInvalidChanges = errors.New("the change is invalid")
	ErrNoCommonSnapshot  = errors.New("trees doesn't have a common snapshot")
	ErrNoChangeInTree    = errors.New("no such change in tree")
	ErrMissingKey        = errors.New("missing current read key")
	ErrDerived           = errors.New("expect >= 2 changes in derived tree")
	ErrDeleted           = errors.New("object tree is deleted")
)

type AddResultSummary int

type AddResult struct {
	OldHeads []string
	Heads    []string
	Added    []*treechangeproto.RawTreeChangeWithId

	Mode Mode
}

type RawChangesPayload struct {
	NewHeads   []string
	RawChanges []*treechangeproto.RawTreeChangeWithId
}

type ChangeIterateFunc = func(change *Change) bool
type ChangeConvertFunc = func(change *Change, decrypted []byte) (any, error)

type ReadableObjectTree interface {
	RWLocker

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
	ChangesAfterCommonSnapshot(snapshotPath, heads []string) ([]*treechangeproto.RawTreeChangeWithId, error)

	Storage() treestorage.TreeStorage

	AddContent(ctx context.Context, content SignableChangeContent) (AddResult, error)
	AddRawChanges(ctx context.Context, changes RawChangesPayload) (AddResult, error)

	UnpackChange(raw *treechangeproto.RawTreeChangeWithId) (data []byte, err error)
	PrepareChange(content SignableChangeContent) (res *treechangeproto.RawTreeChangeWithId, err error)

	Delete() error
	Close() error
	TryClose(objectTTL time.Duration) (bool, error)
}

type objectTree struct {
	treeStorage     treestorage.TreeStorage
	changeBuilder   ChangeBuilder
	validator       ObjectTreeValidator
	rawChangeLoader *rawChangeLoader
	treeBuilder     *treeBuilder
	aclList         list.AclList

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

	sync.RWMutex
}

func (ot *objectTree) rebuildFromStorage(theirHeads []string, newChanges []*Change) (err error) {
	oldTree := ot.tree
	ot.treeBuilder.Reset()
	ot.tree, err = ot.treeBuilder.Build(theirHeads, newChanges)
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

func (ot *objectTree) AddContent(ctx context.Context, content SignableChangeContent) (res AddResult, err error) {
	if ot.isDeleted {
		err = ErrDeleted
		return
	}
	payload, err := ot.prepareBuilderContent(content)
	if err != nil {
		return
	}

	// saving old heads
	oldHeads := make([]string, 0, len(ot.tree.Heads()))
	oldHeads = append(oldHeads, ot.tree.Heads()...)

	objChange, rawChange, err := ot.changeBuilder.Build(payload)
	if content.IsSnapshot {
		// clearing tree, because we already saved everything in the last snapshot
		ot.tree = &Tree{}
	}
	err = ot.tree.AddMergedHead(objChange)
	if err != nil {
		panic(err)
	}

	err = ot.treeStorage.AddRawChangesSetHeads([]*treechangeproto.RawTreeChangeWithId{rawChange}, []string{objChange.Id})
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
		Added:    []*treechangeproto.RawTreeChangeWithId{rawChange},
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

func (ot *objectTree) AddRawChanges(ctx context.Context, changesPayload RawChangesPayload) (addResult AddResult, err error) {
	if ot.isDeleted {
		err = ErrDeleted
		return
	}
	lastHeadId := ot.tree.lastIteratedHeadId
	addResult, err = ot.addRawChanges(ctx, changesPayload)
	if err != nil {
		return
	}

	// reducing tree if we have new roots
	ot.tree.reduceTree()

	// that means that we removed the ids while reducing
	if _, exists := ot.tree.attached[lastHeadId]; !exists {
		addResult.Mode = Rebuild
	}

	err = ot.treeStorage.AddRawChangesSetHeads(addResult.Added, addResult.Heads)
	if err != nil {
		// rolling back all changes made to inmemory state
		ot.rebuildFromStorage(nil, nil)
	}
	return
}

func (ot *objectTree) addRawChanges(ctx context.Context, changesPayload RawChangesPayload) (addResult AddResult, err error) {
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
	filteredHeads, ot.newChangesBuf, ot.newSnapshotsBuf, ot.notSeenIdxBuf = ot.validator.FilterChanges(ot.aclList, changesPayload.NewHeads, ot.newChangesBuf, ot.newSnapshotsBuf, ot.notSeenIdxBuf)
	if filteredHeads {
		// if we filtered some of the heads, then we don't know which heads to use
		headsToUse = []string{}
	}
	rollback := func(changes []*Change) {
		for _, ch := range changes {
			if _, exists := ot.tree.attached[ch.Id]; exists {
				delete(ot.tree.attached, ch.Id)
			}
		}
		ot.tree.headIds = headsCopy(prevHeadsCopy)
		ot.tree.lastIteratedHeadId = lastIteratedId
	}

	// checks if we need to go to database
	snapshotNotInTree := func(ch *Change) bool {
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
	for _, ch := range ot.newChangesBuf {
		if snapshotNotInTree(ch) {
			shouldRebuildFromStorage = true
			break
		}
	}
	log := log.With(zap.String("treeId", ot.id))
	if shouldRebuildFromStorage {
		err = ot.rebuildFromStorage(headsToUse, ot.newChangesBuf)
		if err != nil {
			log.Error("failed to rebuild with new heads", zap.Strings("headsToUse", headsToUse), zap.Error(err))
			// rebuilding without new changes
			rebuildErr := ot.rebuildFromStorage(nil, nil)
			if rebuildErr != nil {
				log.Error("failed to rebuild from storage", zap.Strings("heads", ot.Heads()), zap.Error(rebuildErr))
			}
			return
		}
		addResult, err = ot.createAddResult(prevHeadsCopy, Rebuild, nil, changesPayload.RawChanges)
		if err != nil {
			log.Error("failed to create add result", zap.Strings("headsToUse", headsToUse), zap.Error(err))
			// that means that some unattached changes were somehow corrupted in memory
			// this shouldn't happen but if that happens, then rebuilding from storage
			rebuildErr := ot.rebuildFromStorage(nil, nil)
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
		addResult, err = ot.createAddResult(prevHeadsCopy, mode, treeChangesAdded, changesPayload.RawChanges)
		if err != nil {
			// that means that some unattached changes were somehow corrupted in memory
			// this shouldn't happen but if that happens, then rebuilding from storage
			ot.rebuildFromStorage(nil, nil)
			return
		}
		return
	}
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
				raw, err = ot.changeBuilder.Marshall(ch)
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
	if !ot.treeBuilder.keepInMemoryData {
		for _, ch := range treeChangesAdded {
			ch.Data = nil
		}
	}
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
		ot.tree.Iterate(id, iterate)
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

	ot.tree.Iterate(id, func(c *Change) (isContinue bool) {
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

	return ot.rawChangeLoader.Load(commonSnapshot, ot.tree, theirHeads)
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

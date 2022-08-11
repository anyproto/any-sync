package tree

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage/treepb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"go.uber.org/zap"
	"sync"
)

type AddResultSummary int

var ErrTreeWithoutIdentity = errors.New("acl tree is created without identity")

const (
	AddResultSummaryNothing AddResultSummary = iota
	AddResultSummaryAppend
	AddResultSummaryRebuild
)

type AddResult struct {
	OldHeads []string
	Heads    []string
	Added    []*aclpb.RawChange
	// TODO: add summary for changes
	Summary AddResultSummary
}

type ACLTreeUpdateListener interface {
	Update(tree ACLTree)
	Rebuild(tree ACLTree)
}

type NoOpListener struct{}

func (n NoOpListener) Update(tree ACLTree) {}

func (n NoOpListener) Rebuild(tree ACLTree) {}

type RWLocker interface {
	sync.Locker
	RLock()
	RUnlock()
}

var ErrNoCommonSnapshot = errors.New("trees doesn't have a common snapshot")

type ACLTree interface {
	RWLocker
	CommonTree

	ACLState() *ACLState
	AddContent(ctx context.Context, f func(builder ACLChangeBuilder) error) (*aclpb.RawChange, error)
	AddRawChanges(ctx context.Context, changes ...*aclpb.RawChange) (AddResult, error)
}

type aclTree struct {
	treeStorage    treestorage.TreeStorage
	accountData    *account.AccountData
	updateListener ACLTreeUpdateListener

	id       string
	header   *treepb.TreeHeader
	tree     *Tree
	aclState *ACLState

	treeBuilder     *treeBuilder
	aclStateBuilder *aclStateBuilder
	changeBuilder   *aclChangeBuilder

	sync.RWMutex
}

func BuildACLTreeWithIdentity(t treestorage.TreeStorage, acc *account.AccountData, listener ACLTreeUpdateListener) (ACLTree, error) {
	treeBuilder := newTreeBuilder(t, acc.Decoder)
	aclStateBuilder := newACLStateBuilderWithIdentity(acc.Decoder, acc)
	changeBuilder := newACLChangeBuilder()

	aclTree := &aclTree{
		treeStorage:     t,
		accountData:     acc,
		tree:            nil,
		aclState:        nil,
		treeBuilder:     treeBuilder,
		aclStateBuilder: aclStateBuilder,
		changeBuilder:   changeBuilder,
		updateListener:  listener,
	}
	err := aclTree.rebuildFromStorage()
	if err != nil {
		return nil, err
	}
	err = aclTree.removeOrphans()
	if err != nil {
		return nil, err
	}
	err = t.SetHeads(aclTree.Heads())
	if err != nil {
		return nil, err
	}
	aclTree.id, err = t.TreeID()
	if err != nil {
		return nil, err
	}
	aclTree.header, err = t.Header()
	if err != nil {
		return nil, err
	}

	if listener != nil {
		listener.Rebuild(aclTree)
	}

	return aclTree, nil
}

func BuildACLTree(t treestorage.TreeStorage, decoder signingkey.PubKeyDecoder, listener ACLTreeUpdateListener) (ACLTree, error) {
	treeBuilder := newTreeBuilder(t, decoder)
	aclStateBuilder := newACLStateBuilder()
	changeBuilder := newACLChangeBuilder()

	aclTree := &aclTree{
		treeStorage:     t,
		tree:            nil,
		aclState:        nil,
		treeBuilder:     treeBuilder,
		aclStateBuilder: aclStateBuilder,
		changeBuilder:   changeBuilder,
		updateListener:  listener,
	}
	err := aclTree.rebuildFromStorage()
	if err != nil {
		return nil, err
	}
	err = aclTree.removeOrphans()
	if err != nil {
		return nil, err
	}
	err = t.SetHeads(aclTree.Heads())
	if err != nil {
		return nil, err
	}
	aclTree.id, err = t.TreeID()
	if err != nil {
		return nil, err
	}
	aclTree.header, err = t.Header()
	if err != nil {
		return nil, err
	}

	if listener != nil {
		listener.Rebuild(aclTree)
	}

	return aclTree, nil
}

func (a *aclTree) removeOrphans() error {
	// removing attached or invalid orphans
	var toRemove []string

	orphans, err := a.treeStorage.Orphans()
	if err != nil {
		return err
	}
	for _, orphan := range orphans {
		if _, exists := a.tree.attached[orphan]; exists {
			toRemove = append(toRemove, orphan)
		}
		if _, exists := a.tree.invalidChanges[orphan]; exists {
			toRemove = append(toRemove, orphan)
		}
	}
	return a.treeStorage.RemoveOrphans(toRemove...)
}

func (a *aclTree) rebuildFromStorage() (err error) {
	a.treeBuilder.Init()

	a.tree, err = a.treeBuilder.Build(true)
	if err != nil {
		return err
	}

	err = a.aclStateBuilder.Init(a.tree)
	if err != nil {
		return err
	}

	a.aclState, err = a.aclStateBuilder.Build()
	if err != nil {
		return err
	}

	return nil
}

func (a *aclTree) ID() string {
	return a.id
}

func (a *aclTree) Header() *treepb.TreeHeader {
	return a.header
}

func (a *aclTree) ACLState() *ACLState {
	return a.aclState
}

func (a *aclTree) Storage() treestorage.TreeStorage {
	return a.treeStorage
}

func (a *aclTree) AddContent(ctx context.Context, build func(builder ACLChangeBuilder) error) (*aclpb.RawChange, error) {
	if a.accountData == nil {
		return nil, ErrTreeWithoutIdentity
	}

	defer func() {
		// TODO: should this be called in a separate goroutine to prevent accidental cycles (tree->updater->tree)
		if a.updateListener != nil {
			a.updateListener.Update(a)
		}
	}()

	a.changeBuilder.Init(a.aclState, a.tree, a.accountData)
	err := build(a.changeBuilder)
	if err != nil {
		return nil, err
	}

	ch, marshalled, err := a.changeBuilder.BuildAndApply()
	if err != nil {
		return nil, err
	}
	a.tree.AddFast(ch)
	rawCh := &aclpb.RawChange{
		Payload:   marshalled,
		Signature: ch.Signature(),
		Id:        ch.Id,
	}

	err = a.treeStorage.AddRawChange(rawCh)
	if err != nil {
		return nil, err
	}

	err = a.treeStorage.SetHeads([]string{ch.Id})
	if err != nil {
		return nil, err
	}
	return rawCh, nil
}

func (a *aclTree) AddRawChanges(ctx context.Context, rawChanges ...*aclpb.RawChange) (AddResult, error) {
	// TODO: make proper error handling, because there are a lot of corner cases where this will break
	var err error
	var mode Mode

	var changes []*Change // TODO: = addChangesBuf[:0] ...
	var notSeenIdx []int
	prevHeads := a.tree.Heads()
	for idx, ch := range rawChanges {
		if a.HasChange(ch.Id) {
			continue
		}

		change, err := NewFromRawChange(ch)
		// TODO: think what if we will have incorrect signatures on rawChanges, how everything will work
		if err != nil {
			continue
		}
		changes = append(changes, change)
		notSeenIdx = append(notSeenIdx, idx)
	}

	if len(notSeenIdx) == 0 {
		return AddResult{
			OldHeads: prevHeads,
			Heads:    prevHeads,
			Summary:  AddResultSummaryNothing,
		}, nil
	}

	defer func() {
		if err != nil {
			return
		}

		err = a.removeOrphans()
		if err != nil {
			return
		}

		err = a.treeStorage.SetHeads(a.tree.Heads())
		if err != nil {
			return
		}

		if a.updateListener == nil {
			return
		}

		switch mode {
		case Append:
			a.updateListener.Update(a)
		case Rebuild:
			a.updateListener.Rebuild(a)
		default:
			break
		}
	}()

	getAddedChanges := func() []*aclpb.RawChange {
		var added []*aclpb.RawChange
		for _, idx := range notSeenIdx {
			rawChange := rawChanges[idx]
			if _, exists := a.tree.attached[rawChange.Id]; exists {
				added = append(added, rawChange)
			}
		}
		return added
	}

	for _, ch := range changes {
		err = a.treeStorage.AddChange(ch)
		if err != nil {
			return AddResult{}, err
		}
		err = a.treeStorage.AddOrphans(ch.Id)
		if err != nil {
			return AddResult{}, err
		}
	}

	rebuild := func() (AddResult, error) {
		err = a.rebuildFromStorage()
		if err != nil {
			return AddResult{}, err
		}

		return AddResult{
			OldHeads: prevHeads,
			Heads:    a.tree.Heads(),
			Added:    getAddedChanges(),
			Summary:  AddResultSummaryRebuild,
		}, nil
	}

	mode = a.tree.Add(changes...)
	switch mode {
	case Nothing:
		for _, ch := range changes {
			// rebuilding if the snapshot is different from the root
			if ch.SnapshotId != a.tree.RootId() && ch.SnapshotId != "" {
				return rebuild()
			}
		}

		return AddResult{
			OldHeads: prevHeads,
			Heads:    prevHeads,
			Summary:  AddResultSummaryNothing,
		}, nil

	case Rebuild:
		return rebuild()
	default:
		// just rebuilding the state from start without reloading everything from tree storage
		// as an optimization we could've started from current heads, but I didn't implement that
		a.aclState, err = a.aclStateBuilder.Build()
		if err != nil {
			return AddResult{}, err
		}

		return AddResult{
			OldHeads: prevHeads,
			Heads:    a.tree.Heads(),
			Added:    getAddedChanges(),
			Summary:  AddResultSummaryAppend,
		}, nil
	}
}

func (a *aclTree) Iterate(f func(change *Change) bool) {
	a.tree.Iterate(a.tree.RootId(), f)
}

func (a *aclTree) IterateFrom(s string, f func(change *Change) bool) {
	a.tree.Iterate(s, f)
}

func (a *aclTree) HasChange(s string) bool {
	_, attachedExists := a.tree.attached[s]
	_, unattachedExists := a.tree.unAttached[s]
	_, invalidExists := a.tree.invalidChanges[s]
	return attachedExists || unattachedExists || invalidExists
}

func (a *aclTree) Heads() []string {
	return a.tree.Heads()
}

func (a *aclTree) Root() *Change {
	return a.tree.Root()
}

func (a *aclTree) Close() error {
	return nil
}

func (a *aclTree) SnapshotPath() []string {
	// TODO: think about caching this

	var path []string
	// TODO: think that the user may have not all of the snapshots locally
	currentSnapshotId := a.tree.RootId()
	for currentSnapshotId != "" {
		sn, err := a.treeBuilder.loadChange(currentSnapshotId)
		if err != nil {
			break
		}
		path = append(path, currentSnapshotId)
		currentSnapshotId = sn.SnapshotId
	}
	return path
}

func (a *aclTree) ChangesAfterCommonSnapshot(theirPath []string) ([]*aclpb.RawChange, error) {
	// TODO: think about when the clients will have their full acl tree and thus full snapshots
	//  but no changes after some of the snapshots

	var (
		isNewDocument = len(theirPath) == 0
		ourPath       = a.SnapshotPath()
		// by default returning everything we have
		commonSnapshot = ourPath[len(ourPath)-1] // TODO: root snapshot, probably it is better to have a specific method in treestorage
		err            error
	)

	// if this is non-empty request
	if !isNewDocument {
		commonSnapshot, err = commonSnapshotForTwoPaths(ourPath, theirPath)
		if err != nil {
			return nil, err
		}
	}
	var rawChanges []*aclpb.RawChange
	// using custom load function to skip verification step and save raw changes
	load := func(id string) (*Change, error) {
		raw, err := a.treeStorage.GetChange(context.Background(), id)
		if err != nil {
			return nil, err
		}

		aclChange, err := a.treeBuilder.makeUnverifiedACLChange(raw)
		if err != nil {
			return nil, err
		}

		ch := NewChange(id, aclChange)
		rawChanges = append(rawChanges, raw)
		return ch, nil
	}
	// we presume that we have everything after the common snapshot, though this may not be the case in case of clients and only ACL tree changes
	log.With(
		zap.Strings("heads", a.tree.Heads()),
		zap.String("breakpoint", commonSnapshot),
		zap.String("id", a.id)).
		Debug("getting all changes from common snapshot")
	_, err = a.treeBuilder.dfs(a.tree.Heads(), commonSnapshot, load)
	if err != nil {
		return nil, err
	}
	if isNewDocument {
		// adding snapshot to raw changes
		_, err = load(commonSnapshot)
		if err != nil {
			return nil, err
		}
	}
	log.With(
		zap.Int("len(changes)", len(rawChanges)),
		zap.String("id", a.id)).
		Debug("returning all changes after common snapshot")

	return rawChanges, nil
}

func (a *aclTree) DebugDump() (string, error) {
	return a.tree.Graph(ACLDescriptionParser)
}

func commonSnapshotForTwoPaths(ourPath []string, theirPath []string) (string, error) {
	var i int
	var j int
	log.With(zap.Strings("our path", ourPath), zap.Strings("their path", theirPath)).
		Debug("finding common snapshot for two paths")
OuterLoop:
	// find starting point from the right
	for i = len(ourPath) - 1; i >= 0; i-- {
		for j = len(theirPath) - 1; j >= 0; j-- {
			// most likely there would be only one comparison, because mostly the snapshot path will start from the root for nodes
			if ourPath[i] == theirPath[j] {
				break OuterLoop
			}
		}
	}
	if i < 0 || j < 0 {
		return "", ErrNoCommonSnapshot
	}
	// find last common element of the sequence moving from right to left
	for i >= 0 && j >= 0 {
		if ourPath[i] == theirPath[j] {
			i--
			j--
		}
	}
	return ourPath[i+1], nil
}

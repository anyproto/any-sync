package tree

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"sync"
	"time"
)

type TreeUpdateListener interface {
	Update(tree DocTree)
	Rebuild(tree DocTree)
}

type RWLocker interface {
	sync.Locker
	RLock()
	RUnlock()
}

var ErrHasInvalidChanges = errors.New("the change is invalid")
var ErrNoCommonSnapshot = errors.New("trees doesn't have a common snapshot")
var ErrTreeWithoutIdentity = errors.New("acl tree is created without identity")

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
	// TODO: add summary for changes
	Summary AddResultSummary
}

type DocTree interface {
	RWLocker
	CommonTree
	AddContent(ctx context.Context, aclList list.ACLList, content proto.Marshaler, isSnapshot bool) (*aclpb.RawChange, error)
	AddRawChanges(ctx context.Context, aclList list.ACLList, changes ...*aclpb.RawChange) (AddResult, error)
}

type docTree struct {
	treeStorage    treestorage.TreeStorage
	accountData    *account.AccountData
	updateListener TreeUpdateListener

	id     string
	header *aclpb.Header
	tree   *Tree

	treeBuilder *treeBuilder
	validator   DocTreeValidator

	difSnapshotBuf []*aclpb.RawChange
	tmpChangesBuf  []*Change
	notSeenIdxBuf  []int

	identityKeys map[string]signingkey.PubKey

	sync.RWMutex
}

func BuildDocTreeWithIdentity(t treestorage.TreeStorage, acc *account.AccountData, listener TreeUpdateListener, aclList list.ACLList) (DocTree, error) {
	treeBuilder := newTreeBuilder(t, acc.Decoder)
	validator := newTreeValidator()

	docTree := &docTree{
		treeStorage:    t,
		accountData:    acc,
		tree:           nil,
		treeBuilder:    treeBuilder,
		validator:      validator,
		updateListener: listener,
		tmpChangesBuf:  make([]*Change, 0, 10),
		difSnapshotBuf: make([]*aclpb.RawChange, 0, 10),
		notSeenIdxBuf:  make([]int, 0, 10),
		identityKeys:   make(map[string]signingkey.PubKey),
	}
	err := docTree.rebuildFromStorage(aclList, nil)
	if err != nil {
		return nil, err
	}
	docTree.id, err = t.ID()
	if err != nil {
		return nil, err
	}
	docTree.header, err = t.Header()
	if err != nil {
		return nil, err
	}

	if listener != nil {
		listener.Rebuild(docTree)
	}

	return docTree, nil
}

func BuildDocTree(t treestorage.TreeStorage, decoder keys.Decoder, listener TreeUpdateListener, aclList list.ACLList) (DocTree, error) {
	treeBuilder := newTreeBuilder(t, decoder)
	validator := newTreeValidator()

	docTree := &docTree{
		treeStorage:    t,
		tree:           nil,
		treeBuilder:    treeBuilder,
		validator:      validator,
		updateListener: listener,
		tmpChangesBuf:  make([]*Change, 0, 10),
		difSnapshotBuf: make([]*aclpb.RawChange, 0, 10),
		notSeenIdxBuf:  make([]int, 0, 10),
		identityKeys:   make(map[string]signingkey.PubKey),
	}
	err := docTree.rebuildFromStorage(aclList, nil)
	if err != nil {
		return nil, err
	}
	storageHeads, err := t.Heads()
	if err != nil {
		return nil, err
	}
	// comparing rebuilt heads with heads in storage
	// in theory it can happen that we didn't set heads because the process has crashed
	// therefore we want to set them later
	if !slice.UnsortedEquals(storageHeads, docTree.tree.Heads()) {
		log.With(zap.Strings("storage", storageHeads), zap.Strings("rebuilt", docTree.tree.Heads())).
			Errorf("the heads in storage and tree are different")
		err = t.SetHeads(docTree.tree.Heads())
		if err != nil {
			return nil, err
		}
	}

	docTree.id, err = t.ID()
	if err != nil {
		return nil, err
	}
	docTree.header, err = t.Header()
	if err != nil {
		return nil, err
	}

	if listener != nil {
		listener.Rebuild(docTree)
	}

	return docTree, nil
}

func (d *docTree) rebuildFromStorage(aclList list.ACLList, newChanges []*Change) (err error) {
	d.treeBuilder.Init(d.identityKeys)

	d.tree, err = d.treeBuilder.Build(false, newChanges)
	if err != nil {
		return err
	}

	return d.validator.ValidateTree(d.tree, aclList)
}

func (d *docTree) ID() string {
	return d.id
}

func (d *docTree) Header() *aclpb.Header {
	return d.header
}

func (d *docTree) Storage() treestorage.TreeStorage {
	return d.treeStorage
}

func (d *docTree) AddContent(ctx context.Context, aclList list.ACLList, content proto.Marshaler, isSnapshot bool) (*aclpb.RawChange, error) {
	if d.accountData == nil {
		return nil, ErrTreeWithoutIdentity
	}

	defer func() {
		// TODO: should this be called in a separate goroutine to prevent accidental cycles (tree->updater->tree)
		if d.updateListener != nil {
			d.updateListener.Update(d)
		}
	}()
	state := aclList.ACLState()
	change := &aclpb.Change{
		TreeHeadIds:        d.tree.Heads(),
		AclHeadId:          aclList.Head().Id,
		SnapshotBaseId:     d.tree.RootId(),
		CurrentReadKeyHash: state.CurrentReadKeyHash(),
		Timestamp:          int64(time.Now().Nanosecond()),
		Identity:           d.accountData.Identity,
		IsSnapshot:         isSnapshot,
	}

	marshalledData, err := content.Marshal()
	if err != nil {
		return nil, err
	}
	encrypted, err := state.UserReadKeys()[state.CurrentReadKeyHash()].Encrypt(marshalledData)
	if err != nil {
		return nil, err
	}
	change.ChangesData = encrypted

	fullMarshalledChange, err := proto.Marshal(change)
	if err != nil {
		return nil, err
	}
	signature, err := d.accountData.SignKey.Sign(fullMarshalledChange)
	if err != nil {
		return nil, err
	}
	id, err := cid.NewCIDFromBytes(fullMarshalledChange)
	if err != nil {
		return nil, err
	}
	ch := NewChange(id, change)
	ch.ParsedModel = content
	ch.Sign = signature

	if isSnapshot {
		// clearing tree, because we already fixed everything in the last snapshot
		d.tree = &Tree{}
	}
	d.tree.AddFast(ch) // TODO: Add head
	rawCh := &aclpb.RawChange{
		Payload:   fullMarshalledChange,
		Signature: ch.Signature(),
		Id:        ch.Id,
	}

	err = d.treeStorage.AddRawChange(rawCh)
	if err != nil {
		return nil, err
	}

	err = d.treeStorage.SetHeads([]string{ch.Id})
	if err != nil {
		return nil, err
	}
	return rawCh, nil
}

func (d *docTree) AddRawChanges(ctx context.Context, aclList list.ACLList, rawChanges ...*aclpb.RawChange) (addResult AddResult, err error) {
	var mode Mode

	// resetting buffers
	d.tmpChangesBuf = d.tmpChangesBuf[:0]
	d.notSeenIdxBuf = d.notSeenIdxBuf[:0]
	d.difSnapshotBuf = d.difSnapshotBuf[:0]

	prevHeads := d.tree.Heads()

	// filtering changes, verifying and unmarshalling them
	for idx, ch := range rawChanges {
		if d.HasChange(ch.Id) {
			continue
		}
		// if we already added the change to invalid ones
		if _, exists := d.tree.invalidChanges[ch.Id]; exists {
			return AddResult{}, ErrHasInvalidChanges
		}

		var change *Change
		change, err = NewFromVerifiedRawChange(ch, d.identityKeys, d.treeBuilder.signingPubKeyDecoder)
		if err != nil {
			return AddResult{}, err
		}

		d.tmpChangesBuf = append(d.tmpChangesBuf, change)
		d.notSeenIdxBuf = append(d.notSeenIdxBuf, idx)
	}

	// if no new changes, then returning
	if len(d.notSeenIdxBuf) == 0 {
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

		// adding to database all the added changes only after they are good
		for _, ch := range addResult.Added {
			err = d.treeStorage.AddRawChange(ch)
			if err != nil {
				return
			}
		}

		// setting heads
		err = d.treeStorage.SetHeads(d.tree.Heads())
		if err != nil {
			return
		}

		if d.updateListener == nil {
			return
		}

		switch mode {
		case Append:
			d.updateListener.Update(d)
		case Rebuild:
			d.updateListener.Rebuild(d)
		default:
			break
		}
	}()

	// returns changes that we added to the tree
	getAddedChanges := func() []*aclpb.RawChange {
		var added []*aclpb.RawChange
		for _, idx := range d.notSeenIdxBuf {
			rawChange := rawChanges[idx]
			if _, exists := d.tree.attached[rawChange.Id]; exists {
				added = append(added, rawChange)
			}
		}
		return added
	}

	rollback := func() {
		for _, ch := range d.tmpChangesBuf {
			if _, exists := d.tree.attached[ch.Id]; exists {
				delete(d.tree.attached, ch.Id)
			} else if _, exists := d.tree.unAttached[ch.Id]; exists {
				delete(d.tree.unAttached, ch.Id)
			}
		}
	}

	// checking if we have some changes with different snapshot and then rebuilding
	for _, ch := range d.tmpChangesBuf {
		if ch.SnapshotId != d.tree.RootId() && ch.SnapshotId != "" {
			err = d.rebuildFromStorage(aclList, d.tmpChangesBuf)
			if err != nil {
				rollback()
				return AddResult{}, err
			}

			addResult = AddResult{
				OldHeads: prevHeads,
				Heads:    d.tree.Heads(),
				Added:    getAddedChanges(),
				Summary:  AddResultSummaryRebuild,
			}
			err = nil
			return
		}
	}

	// normal mode of operation, where we don't need to rebuild from database
	mode = d.tree.Add(d.tmpChangesBuf...)
	switch mode {
	case Nothing:
		addResult = AddResult{
			OldHeads: prevHeads,
			Heads:    prevHeads,
			Summary:  AddResultSummaryNothing,
		}
		err = nil
		return

	default:
		// just rebuilding the state from start without reloading everything from tree storage
		// as an optimization we could've started from current heads, but I didn't implement that
		err = d.validator.ValidateTree(d.tree, aclList)
		if err != nil {
			rollback()
			return AddResult{}, ErrHasInvalidChanges
		}

		addResult = AddResult{
			OldHeads: prevHeads,
			Heads:    d.tree.Heads(),
			Added:    getAddedChanges(),
			Summary:  AddResultSummaryAppend,
		}
		err = nil
	}
	return
}

func (d *docTree) Iterate(f func(change *Change) bool) {
	d.tree.Iterate(d.tree.RootId(), f)
}

func (d *docTree) IterateFrom(s string, f func(change *Change) bool) {
	d.tree.Iterate(s, f)
}

func (d *docTree) HasChange(s string) bool {
	_, attachedExists := d.tree.attached[s]
	_, unattachedExists := d.tree.unAttached[s]
	return attachedExists || unattachedExists
}

func (d *docTree) Heads() []string {
	return d.tree.Heads()
}

func (d *docTree) Root() *Change {
	return d.tree.Root()
}

func (d *docTree) Close() error {
	return nil
}

func (d *docTree) SnapshotPath() []string {
	// TODO: think about caching this

	var path []string
	// TODO: think that the user may have not all of the snapshots locally
	currentSnapshotId := d.tree.RootId()
	for currentSnapshotId != "" {
		sn, err := d.treeBuilder.loadChange(currentSnapshotId)
		if err != nil {
			break
		}
		path = append(path, currentSnapshotId)
		currentSnapshotId = sn.SnapshotId
	}
	return path
}

func (d *docTree) ChangesAfterCommonSnapshot(theirPath []string) ([]*aclpb.RawChange, error) {
	// TODO: think about when the clients will have their full acl tree and thus full snapshots
	//  but no changes after some of the snapshots

	var (
		isNewDocument = len(theirPath) == 0
		ourPath       = d.SnapshotPath()
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
	// TODO: if the snapshot is in the tree we probably can skip going to the DB
	var rawChanges []*aclpb.RawChange
	// using custom load function to skip verification step and save raw changes
	load := func(id string) (*Change, error) {
		raw, err := d.treeStorage.GetRawChange(context.Background(), id)
		if err != nil {
			return nil, err
		}

		ch, err := NewFromRawChange(raw)
		if err != nil {
			return nil, err
		}

		rawChanges = append(rawChanges, raw)
		return ch, nil
	}
	// we presume that we have everything after the common snapshot, though this may not be the case in case of clients and only ACL tree changes
	log.With(
		zap.Strings("heads", d.tree.Heads()),
		zap.String("breakpoint", commonSnapshot),
		zap.String("id", d.id)).
		Debug("getting all changes from common snapshot")
	_, err = d.treeBuilder.dfs(d.tree.Heads(), commonSnapshot, load)
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
		zap.String("id", d.id)).
		Debug("returning all changes after common snapshot")

	return rawChanges, nil
}

func (d *docTree) DebugDump() (string, error) {
	return d.tree.Graph(NoOpDescriptionParser)
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

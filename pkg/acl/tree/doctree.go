package tree

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage/treepb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"sync"
	"time"
)

type TreeUpdateListener interface {
	Update(tree DocTree)
	Rebuild(tree DocTree)
}

type DocTree interface {
	RWLocker
	CommonTree
	AddContent(ctx context.Context, aclTree ACLTree, content proto.Marshaler, isSnapshot bool) (*aclpb.RawChange, error)
	AddRawChanges(ctx context.Context, aclTree ACLTree, changes ...*aclpb.RawChange) (AddResult, error)
}

type docTree struct {
	treeStorage    treestorage.TreeStorage
	accountData    *account.AccountData
	updateListener TreeUpdateListener

	id     string
	header *treepb.TreeHeader
	tree   *Tree

	treeBuilder *treeBuilder
	validator   DocTreeValidator

	sync.RWMutex
}

func BuildDocTreeWithIdentity(t treestorage.TreeStorage, acc *account.AccountData, listener TreeUpdateListener, aclTree ACLTree) (DocTree, error) {
	treeBuilder := newTreeBuilder(t, acc.Decoder)
	validator := newTreeValidator()

	docTree := &docTree{
		treeStorage:    t,
		accountData:    acc,
		tree:           nil,
		treeBuilder:    treeBuilder,
		validator:      validator,
		updateListener: listener,
	}
	err := docTree.rebuildFromStorage(aclTree)
	if err != nil {
		return nil, err
	}
	err = docTree.removeOrphans()
	if err != nil {
		return nil, err
	}
	err = t.SetHeads(docTree.Heads())
	if err != nil {
		return nil, err
	}
	docTree.id, err = t.TreeID()
	if err != nil {
		return nil, err
	}
	docTree.header, err = t.Header()
	if err != nil {
		return nil, err
	}

	listener.Rebuild(docTree)

	return docTree, nil
}

func BuildDocTree(t treestorage.TreeStorage, decoder signingkey.PubKeyDecoder, listener TreeUpdateListener, aclTree ACLTree) (DocTree, error) {
	treeBuilder := newTreeBuilder(t, decoder)
	validator := newTreeValidator()

	docTree := &docTree{
		treeStorage:    t,
		tree:           nil,
		treeBuilder:    treeBuilder,
		validator:      validator,
		updateListener: listener,
	}
	err := docTree.rebuildFromStorage(aclTree)
	if err != nil {
		return nil, err
	}
	err = docTree.removeOrphans()
	if err != nil {
		return nil, err
	}
	err = t.SetHeads(docTree.Heads())
	if err != nil {
		return nil, err
	}
	docTree.id, err = t.TreeID()
	if err != nil {
		return nil, err
	}
	docTree.header, err = t.Header()
	if err != nil {
		return nil, err
	}

	listener.Rebuild(docTree)

	return docTree, nil
}

func (d *docTree) removeOrphans() error {
	// removing attached or invalid orphans
	var toRemove []string

	orphans, err := d.treeStorage.Orphans()
	if err != nil {
		return err
	}
	for _, orphan := range orphans {
		if _, exists := d.tree.attached[orphan]; exists {
			toRemove = append(toRemove, orphan)
		}
		if _, exists := d.tree.invalidChanges[orphan]; exists {
			toRemove = append(toRemove, orphan)
		}
	}
	return d.treeStorage.RemoveOrphans(toRemove...)
}

func (d *docTree) rebuildFromStorage(aclTree ACLTree) (err error) {
	d.treeBuilder.Init()

	d.tree, err = d.treeBuilder.Build(false)
	if err != nil {
		return err
	}

	return d.validator.ValidateTree(d.tree, aclTree)
}

func (d *docTree) ID() string {
	return d.id
}

func (d *docTree) Header() *treepb.TreeHeader {
	return d.header
}

func (d *docTree) Storage() treestorage.TreeStorage {
	return d.treeStorage
}

func (d *docTree) AddContent(ctx context.Context, aclTree ACLTree, content proto.Marshaler, isSnapshot bool) (*aclpb.RawChange, error) {
	if d.accountData == nil {
		return nil, ErrTreeWithoutIdentity
	}

	defer func() {
		// TODO: should this be called in a separate goroutine to prevent accidental cycles (tree->updater->tree)
		d.updateListener.Update(d)
	}()
	state := aclTree.ACLState()
	change := &aclpb.Change{
		TreeHeadIds:        d.tree.Heads(),
		AclHeadIds:         aclTree.Heads(),
		SnapshotBaseId:     d.tree.RootId(),
		CurrentReadKeyHash: state.currentReadKeyHash,
		Timestamp:          int64(time.Now().Nanosecond()),
		Identity:           d.accountData.Identity,
	}

	marshalledData, err := content.Marshal()
	if err != nil {
		return nil, err
	}
	encrypted, err := state.userReadKeys[state.currentReadKeyHash].Encrypt(marshalledData)
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

func (d *docTree) AddRawChanges(ctx context.Context, aclTree ACLTree, rawChanges ...*aclpb.RawChange) (AddResult, error) {
	// TODO: make proper error handling, because there are a lot of corner cases where this will break
	var err error
	var mode Mode

	var changes []*Change // TODO: = addChangesBuf[:0] ...
	for _, ch := range rawChanges {
		change, err := NewFromRawChange(ch)
		// TODO: think what if we will have incorrect signatures on rawChanges, how everything will work
		if err != nil {
			continue
		}
		changes = append(changes, change)
	}

	defer func() {
		if err != nil {
			return
		}

		err = d.removeOrphans()
		if err != nil {
			return
		}

		err = d.treeStorage.SetHeads(d.tree.Heads())
		if err != nil {
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

	getAddedChanges := func() []*aclpb.RawChange {
		var added []*aclpb.RawChange
		for _, ch := range rawChanges {
			if _, exists := d.tree.attached[ch.Id]; exists {
				added = append(added, ch)
			}
		}
		return added
	}

	prevHeads := d.tree.Heads()
	rebuild := func() (AddResult, error) {
		err = d.rebuildFromStorage(aclTree)
		if err != nil {
			return AddResult{}, err
		}

		return AddResult{
			OldHeads: prevHeads,
			Heads:    d.tree.Heads(),
			Added:    getAddedChanges(),
			Summary:  AddResultSummaryRebuild,
		}, nil
	}

	for _, ch := range changes {
		err = d.treeStorage.AddChange(ch)
		if err != nil {
			return AddResult{}, err
		}
		err = d.treeStorage.AddOrphans(ch.Id)
		if err != nil {
			return AddResult{}, err
		}
	}

	mode = d.tree.Add(changes...)
	switch mode {
	case Nothing:
		for _, ch := range changes {
			// rebuilding if the snapshot is different from the root
			if ch.SnapshotId != d.tree.RootId() {
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
		err = d.validator.ValidateTree(d.tree, aclTree)
		if err != nil {
			return AddResult{}, err
		}

		return AddResult{
			OldHeads: prevHeads,
			Heads:    d.tree.Heads(),
			Added:    getAddedChanges(),
			Summary:  AddResultSummaryAppend,
		}, nil
	}
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
	_, invalidExists := d.tree.invalidChanges[s]
	return attachedExists || unattachedExists || invalidExists
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
	var rawChanges []*aclpb.RawChange
	// using custom load function to skip verification step and save raw changes
	load := func(id string) (*Change, error) {
		raw, err := d.treeStorage.GetChange(context.Background(), id)
		if err != nil {
			return nil, err
		}

		aclChange, err := d.treeBuilder.makeUnverifiedACLChange(raw)
		if err != nil {
			return nil, err
		}

		ch := NewChange(id, aclChange)
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

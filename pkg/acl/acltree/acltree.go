package acltree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"sync"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
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
	// TODO: add summary for changes
	Summary AddResultSummary
}

type TreeUpdateListener interface {
	Update(tree ACLTree)
	Rebuild(tree ACLTree)
}

type ACLTree interface {
	ACLState() *ACLState
	AddContent(f func(builder ChangeBuilder) error) (*Change, error)
	AddChanges(changes ...*Change) (AddResult, error)
	Heads() []string
	Root() *Change
	Iterate(func(change *Change) bool)
	IterateFrom(string, func(change *Change) bool)
	HasChange(string) bool
}

type aclTree struct {
	treeStorage    treestorage.TreeStorage
	accountData    *account.AccountData
	updateListener TreeUpdateListener

	fullTree         *Tree
	aclTreeFromStart *Tree // TODO: right now we don't use it, we can probably have only local var for now. This tree is built from start of the document
	aclState         *ACLState

	treeBuilder       *treeBuilder
	aclTreeBuilder    *aclTreeBuilder
	aclStateBuilder   *aclStateBuilder
	snapshotValidator *snapshotValidator
	changeBuilder     *changeBuilder

	sync.RWMutex
}

func BuildACLTree(
	t treestorage.TreeStorage,
	acc *account.AccountData,
	listener TreeUpdateListener) (ACLTree, error) {
	decoder := keys.NewEd25519Decoder()
	aclTreeBuilder := newACLTreeBuilder(t, decoder)
	treeBuilder := newTreeBuilder(t, decoder)
	snapshotValidator := newSnapshotValidator(decoder, acc)
	aclStateBuilder := newACLStateBuilder(decoder, acc)
	changeBuilder := newChangeBuilder()

	aclTree := &aclTree{
		treeStorage:       t,
		accountData:       acc,
		fullTree:          nil,
		aclState:          nil,
		treeBuilder:       treeBuilder,
		aclTreeBuilder:    aclTreeBuilder,
		aclStateBuilder:   aclStateBuilder,
		snapshotValidator: snapshotValidator,
		changeBuilder:     changeBuilder,
		updateListener:    listener,
	}
	err := aclTree.rebuildFromStorage(false)
	if err != nil {
		return nil, err
	}
	aclTree.removeOrphans()
	t.SetHeads(aclTree.Heads())
	listener.Rebuild(aclTree)

	return aclTree, nil
}

// TODO: this is not used for now, in future we should think about not making full tree rebuild
//func (a *aclTree) rebuildFromTree(validateSnapshot bool) (err error) {
//	if validateSnapshot {
//		err = a.snapshotValidator.Init(a.aclTreeFromStart)
//		if err != nil {
//			return err
//		}
//
//		valid, err := a.snapshotValidator.ValidateSnapshot(a.fullTree.root)
//		if err != nil {
//			return err
//		}
//		if !valid {
//			return a.rebuildFromStorage(true)
//		}
//	}
//
//	err = a.aclStateBuilder.Init(a.fullTree)
//	if err != nil {
//		return err
//	}
//
//	a.aclState, err = a.aclStateBuilder.Build()
//	if err != nil {
//		return err
//	}
//
//	return nil
//}

func (a *aclTree) removeOrphans() {
	// removing attached or invalid orphans
	var toRemove []string

	for _, orphan := range a.treeStorage.Orphans() {
		if _, exists := a.fullTree.attached[orphan]; exists {
			toRemove = append(toRemove, orphan)
		}
		if _, exists := a.fullTree.invalidChanges[orphan]; exists {
			toRemove = append(toRemove, orphan)
		}
	}
	a.treeStorage.RemoveOrphans(toRemove...)
}

func (a *aclTree) rebuildFromStorage(fromStart bool) error {
	a.treeBuilder.Init()
	a.aclTreeBuilder.Init()

	var err error
	a.fullTree, err = a.treeBuilder.Build(fromStart)
	if err != nil {
		return err
	}

	// TODO: remove this from context as this is used only to validate snapshot
	a.aclTreeFromStart, err = a.aclTreeBuilder.Build()
	if err != nil {
		return err
	}

	if !fromStart {
		err = a.snapshotValidator.Init(a.aclTreeFromStart)
		if err != nil {
			return err
		}

		valid, err := a.snapshotValidator.ValidateSnapshot(a.fullTree.root)
		if err != nil {
			return err
		}
		if !valid {
			return a.rebuildFromStorage(true)
		}
	}
	// TODO: there is a question how we can validate not only that the full tree is built correctly
	//  but also that the ACL prev ids are not messed up. I think we should probably compare the resulting
	//  acl state with the acl state which is built in aclTreeFromStart

	err = a.aclStateBuilder.Init(a.fullTree)
	if err != nil {
		return err
	}

	a.aclState, err = a.aclStateBuilder.Build()
	if err != nil {
		return err
	}

	return nil
}

func (a *aclTree) ACLState() *ACLState {
	a.RLock()
	defer a.RUnlock()
	return a.aclState
}

func (a *aclTree) AddContent(build func(builder ChangeBuilder) error) (*Change, error) {
	// TODO: add snapshot creation logic
	a.Lock()
	defer func() {
		a.Unlock()
		// TODO: should this be called in a separate goroutine to prevent accidental cycles (tree->updater->tree)
		a.updateListener.Update(a)
	}()

	a.changeBuilder.Init(a.aclState, a.fullTree, a.accountData)
	err := build(a.changeBuilder)
	if err != nil {
		return nil, err
	}

	ch, marshalled, err := a.changeBuilder.BuildAndApply()
	if err != nil {
		return nil, err
	}
	a.fullTree.AddFast(ch)

	err = a.treeStorage.AddRawChange(&treestorage.RawChange{
		Payload:   marshalled,
		Signature: ch.Signature(),
		Id:        ch.Id,
	})
	if err != nil {
		return nil, err
	}

	a.treeStorage.SetHeads([]string{ch.Id})
	return ch, nil
}

func (a *aclTree) AddChanges(changes ...*Change) (AddResult, error) {
	a.Lock()
	// TODO: make proper error handling, because there are a lot of corner cases where this will break
	var err error
	var mode Mode

	defer func() {
		if err != nil {
			return
		}
		a.removeOrphans()
		a.treeStorage.SetHeads(a.fullTree.Heads())
		a.Unlock()
		switch mode {
		case Append:
			a.updateListener.Update(a)
		case Rebuild:
			a.updateListener.Rebuild(a)
		default:
			break
		}
	}()

	for _, ch := range changes {
		err = a.treeStorage.AddChange(ch)
		if err != nil {
			return AddResult{}, err
		}
		a.treeStorage.AddOrphans(ch.Id)
	}

	prevHeads := a.fullTree.Heads()
	mode = a.fullTree.Add(changes...)
	switch mode {
	case Nothing:
		return AddResult{
			OldHeads: prevHeads,
			Heads:    prevHeads,
			Summary:  AddResultSummaryNothing,
		}, nil

	case Rebuild:
		err = a.rebuildFromStorage(false)
		if err != nil {
			return AddResult{}, err
		}

		return AddResult{
			OldHeads: prevHeads,
			Heads:    a.fullTree.Heads(),
			Summary:  AddResultSummaryRebuild,
		}, nil
	default:
		// just rebuilding the state from start without reloading everything from tree storage
		// as an optimization we could've started from current heads, but I didn't implement that
		a.aclState, err = a.aclStateBuilder.Build()
		if err != nil {
			return AddResult{}, err
		}

		return AddResult{
			OldHeads: prevHeads,
			Heads:    a.fullTree.Heads(),
			Summary:  AddResultSummaryAppend,
		}, nil
	}
}

func (a *aclTree) Iterate(f func(change *Change) bool) {
	a.RLock()
	defer a.RUnlock()
	a.fullTree.Iterate(a.fullTree.RootId(), f)
}

func (a *aclTree) IterateFrom(s string, f func(change *Change) bool) {
	a.RLock()
	defer a.RUnlock()
	a.fullTree.Iterate(s, f)
}

func (a *aclTree) HasChange(s string) bool {
	a.RLock()
	defer a.RUnlock()
	_, attachedExists := a.fullTree.attached[s]
	_, unattachedExists := a.fullTree.unAttached[s]
	_, invalidExists := a.fullTree.invalidChanges[s]
	return attachedExists || unattachedExists || invalidExists
}

func (a *aclTree) Heads() []string {
	a.RLock()
	defer a.RUnlock()
	return a.fullTree.Heads()
}

func (a *aclTree) Root() *Change {
	a.RLock()
	defer a.RUnlock()
	return a.fullTree.Root()
}

package acltree

import (
	"sync"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/thread"
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

type ACLTree interface {
	ACLState() *ACLState
	AddContent(f func(builder ChangeBuilder)) (*Change, error)
	AddChanges(changes ...*Change) (AddResult, error)
	Heads() []string
	Iterate(func(change *Change) bool)
	IterateFrom(string, func(change *Change) bool)
	HasChange(string) bool
}

type aclTree struct {
	thread      thread.Thread
	accountData *account.AccountData

	fullTree         *Tree
	aclTreeFromStart *Tree // TODO: right now we don't use it, we can probably have only local var for now. This tree is built from start of the document
	aclState         *ACLState

	treeBuilder       *treeBuilder
	aclTreeBuilder    *aclTreeBuilder
	aclStateBuilder   *aclStateBuilder
	snapshotValidator *snapshotValidator
	changeBuilder     *changeBuilder

	sync.Mutex
}

func BuildACLTree(t thread.Thread, acc *account.AccountData) (ACLTree, error) {
	decoder := keys.NewEd25519Decoder()
	aclTreeBuilder := newACLTreeBuilder(t, decoder)
	treeBuilder := newTreeBuilder(t, decoder)
	snapshotValidator := newSnapshotValidator(decoder, acc)
	aclStateBuilder := newACLStateBuilder(decoder, acc)
	changeBuilder := newChangeBuilder()

	aclTree := &aclTree{
		thread:            t,
		accountData:       acc,
		fullTree:          nil,
		aclState:          nil,
		treeBuilder:       treeBuilder,
		aclTreeBuilder:    aclTreeBuilder,
		aclStateBuilder:   aclStateBuilder,
		snapshotValidator: snapshotValidator,
		changeBuilder:     changeBuilder,
	}
	err := aclTree.rebuildFromThread(false)
	if err != nil {
		return nil, err
	}

	return aclTree, nil
}

// TODO: this is not used for now, in future we should think about not making full tree rebuild
func (a *aclTree) rebuildFromTree(validateSnapshot bool) (err error) {
	if validateSnapshot {
		err = a.snapshotValidator.Init(a.aclTreeFromStart)
		if err != nil {
			return err
		}

		valid, err := a.snapshotValidator.ValidateSnapshot(a.fullTree.root)
		if err != nil {
			return err
		}
		if !valid {
			return a.rebuildFromThread(true)
		}
	}

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

func (a *aclTree) rebuildFromThread(fromStart bool) error {
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
			return a.rebuildFromThread(true)
		}
	}
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
	return a.aclState
}

func (a *aclTree) AddContent(build func(builder ChangeBuilder)) (*Change, error) {
	// TODO: add snapshot creation logic
	a.Lock()
	defer a.Unlock()

	a.changeBuilder.Init(a.aclState, a.fullTree, a.accountData)
	build(a.changeBuilder)

	ch, marshalled, err := a.changeBuilder.Build()
	if err != nil {
		return nil, err
	}
	err = a.aclState.applyChange(ch.Id, ch.Content)
	if err != nil {
		return nil, err
	}

	a.fullTree.AddFast(ch)

	err = a.thread.AddRawChange(&thread.RawChange{
		Payload:   marshalled,
		Signature: ch.Signature(),
		Id:        ch.Id,
	})
	if err != nil {
		return nil, err
	}

	a.thread.SetHeads([]string{ch.Id})
	return ch, nil
}

func (a *aclTree) AddChanges(changes ...*Change) (AddResult, error) {
	a.Lock()
	defer a.Unlock()
	// TODO: make proper error handling, because there are a lot of corner cases where this will break
	var aclChanges []*Change
	var err error

	defer func() {
		if err != nil {
			return
		}
		// removing attached or invalid orphans
		var toRemove []string

		for _, orphan := range a.thread.Orphans() {
			if _, exists := a.fullTree.attached[orphan]; exists {
				toRemove = append(toRemove, orphan)
			}
			if _, exists := a.fullTree.invalidChanges[orphan]; exists {
				toRemove = append(toRemove, orphan)
			}
		}
		a.thread.RemoveOrphans(toRemove...)
	}()

	for _, ch := range changes {
		if ch.IsACLChange() {
			aclChanges = append(aclChanges, ch)
			break
		}
		err = a.thread.AddChange(ch)
		if err != nil {
			return AddResult{}, err
		}
		a.thread.AddOrphans(ch.Id)
	}

	prevHeads := a.fullTree.Heads()
	mode := a.fullTree.Add(changes...)
	switch mode {
	case Nothing:
		return AddResult{
			OldHeads: prevHeads,
			Heads:    prevHeads,
			Summary:  AddResultSummaryNothing,
		}, nil

	case Rebuild:
		err = a.rebuildFromThread(false)
		if err != nil {
			return AddResult{}, err
		}

		return AddResult{
			OldHeads: prevHeads,
			Heads:    a.fullTree.Heads(),
			Summary:  AddResultSummaryRebuild,
		}, nil
	default:
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
	a.Lock()
	defer a.Unlock()
	a.fullTree.Iterate(a.fullTree.RootId(), f)
}

func (a *aclTree) IterateFrom(s string, f func(change *Change) bool) {
	a.Lock()
	defer a.Unlock()
	a.fullTree.Iterate(s, f)
}

func (a *aclTree) HasChange(s string) bool {
	a.Lock()
	defer a.Unlock()
	_, attachedExists := a.fullTree.attached[s]
	_, unattachedExists := a.fullTree.unAttached[s]
	_, invalidExists := a.fullTree.invalidChanges[s]
	return attachedExists || unattachedExists || invalidExists
}

func (a *aclTree) Heads() []string {
	a.Lock()
	defer a.Unlock()
	return a.fullTree.Heads()
}

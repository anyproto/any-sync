package acltree

import (
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
	AttachedChanges   []*Change
	InvalidChanges    []*Change
	UnattachedChanges []*Change

	Summary AddResultSummary
}

type ACLTree interface {
	ACLState() *ACLState
	AddContent(changeContent *ChangeContent) (*Change, error)
	AddChanges(changes ...*Change) (AddResult, error) // TODO: Make change as interface
	Heads() []string
	Iterate(func(change *Change) bool)
	IterateFrom(string, func(change *Change) bool)
	HasChange(change *Change) bool
}

type aclTree struct {
	thread      thread.Thread
	accountData *account.AccountData

	fullTree *Tree
	aclState *ACLState

	treeBuilder       *treeBuilder
	aclTreeBuilder    *aclTreeBuilder
	aclStateBuilder   *aclStateBuilder
	snapshotValidator *snapshotValidator
}

func BuildACLTree(t thread.Thread, acc *account.AccountData) (ACLTree, error) {
	decoder := keys.NewEd25519Decoder()
	aclTreeBuilder := newACLTreeBuilder(t, decoder)
	treeBuilder := newTreeBuilder(t, decoder)
	snapshotValidator := newSnapshotValidator(decoder, acc)
	aclStateBuilder := newACLStateBuilder(decoder, acc)

	aclTree := &aclTree{
		thread:            t,
		accountData:       acc,
		fullTree:          nil,
		aclState:          nil,
		treeBuilder:       treeBuilder,
		aclTreeBuilder:    aclTreeBuilder,
		aclStateBuilder:   aclStateBuilder,
		snapshotValidator: snapshotValidator,
	}
	err := aclTree.rebuildFromThread(false)
	if err != nil {
		return nil, err
	}

	return aclTree, nil
}

func (a *aclTree) rebuildFromThread(fromStart bool) error {
	var aclTree *Tree

	a.treeBuilder.init()
	a.aclTreeBuilder.init()

	var err error
	a.fullTree, err = a.treeBuilder.build(fromStart)
	if err != nil {
		return err
	}

	// TODO: remove this from context as this is used only to validate snapshot
	aclTree, err = a.aclTreeBuilder.build()
	if err != nil {
		return err
	}

	if !fromStart {
		err = a.snapshotValidator.init(aclTree)
		if err != nil {
			return err
		}

		valid, err := a.snapshotValidator.validateSnapshot(a.fullTree.root)
		if err != nil {
			return err
		}
		if !valid {
			return a.rebuildFromThread(true)
		}
	}
	err = a.aclStateBuilder.init(a.fullTree)
	if err != nil {
		return err
	}

	a.aclState, err = a.aclStateBuilder.build()
	if err != nil {
		return err
	}

	return nil
}

func (a *aclTree) ACLState() *ACLState {
	//TODO implement me
	panic("implement me")
}

func (a *aclTree) AddContent(changeContent *ChangeContent) (*Change, error) {
	//TODO implement me
	panic("implement me")
}

func (a *aclTree) AddChanges(changes ...*Change) (AddResult, error) {
	//TODO implement me
	panic("implement me")
}

func (a *aclTree) Heads() []string {
	//TODO implement me
	panic("implement me")
}

func (a *aclTree) Iterate(f func(change *Change) bool) {
	//TODO implement me
	panic("implement me")
}

func (a *aclTree) IterateFrom(s string, f func(change *Change) bool) {
	//TODO implement me
	panic("implement me")
}

func (a *aclTree) HasChange(change *Change) bool {
	//TODO implement me
	panic("implement me")
}

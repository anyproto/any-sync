package acltree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/aclchanges/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/thread"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
	"github.com/gogo/protobuf/proto"
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
	aclTree  *Tree // this tree is built from start of the document
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

func (a *aclTree) rebuildFromTree(validateSnapshot bool) (err error) {
	if validateSnapshot {
		err = a.snapshotValidator.init(a.aclTree)
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

func (a *aclTree) rebuildFromThread(fromStart bool) error {
	a.treeBuilder.init()
	a.aclTreeBuilder.init()

	var err error
	a.fullTree, err = a.treeBuilder.build(fromStart)
	if err != nil {
		return err
	}

	// TODO: remove this from context as this is used only to validate snapshot
	a.aclTree, err = a.aclTreeBuilder.build()
	if err != nil {
		return err
	}

	if !fromStart {
		err = a.snapshotValidator.init(a.aclTree)
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

// TODO: this should not be the responsibility of ACLTree, move it somewhere else after testing
func (a *aclTree) getACLHeads() []string {
	var aclTreeHeads []string
	for _, head := range a.fullTree.Heads() {
		if slice.FindPos(aclTreeHeads, head) != -1 { // do not scan known heads
			continue
		}
		precedingHeads := a.getPrecedingACLHeads(head)

		for _, aclHead := range precedingHeads {
			if slice.FindPos(aclTreeHeads, aclHead) != -1 {
				continue
			}
			aclTreeHeads = append(aclTreeHeads, aclHead)
		}
	}
	return aclTreeHeads
}

func (a *aclTree) getPrecedingACLHeads(head string) []string {
	headChange := a.fullTree.attached[head]

	if headChange.Content.GetAclData() != nil {
		return []string{head}
	} else {
		return headChange.Content.AclHeadIds
	}
}

func (a *aclTree) ACLState() *ACLState {
	return a.aclState
}

func (a *aclTree) AddContent(changeContent *ChangeContent) (*Change, error) {
	// TODO: add snapshot creation logic
	marshalled, err := changeContent.ChangesData.Marshal()
	if err != nil {
		return nil, err
	}

	encrypted, err := a.aclState.userReadKeys[a.aclState.currentReadKeyHash].
		Encrypt(marshalled)
	if err != nil {
		return nil, err
	}

	aclChange := &pb.ACLChange{
		TreeHeadIds:        a.fullTree.Heads(),
		AclHeadIds:         a.getACLHeads(),
		SnapshotBaseId:     a.fullTree.RootId(),
		AclData:            changeContent.ACLData,
		ChangesData:        encrypted,
		CurrentReadKeyHash: a.aclState.currentReadKeyHash,
		Timestamp:          0,
		Identity:           a.accountData.Identity,
	}

	// TODO: add CID creation logic based on content
	ch := NewChange(changeContent.Id, aclChange)
	ch.DecryptedDocumentChange = marshalled

	fullMarshalledChange, err := proto.Marshal(aclChange)
	if err != nil {
		return nil, err
	}
	signature, err := a.accountData.SignKey.Sign(fullMarshalledChange)
	if err != nil {
		return nil, err
	}

	if aclChange.AclData != nil {
		// we can apply change right away without going through builder, because
		err = a.aclState.applyChange(changeContent.Id, aclChange)
		if err != nil {
			return nil, err
		}
	}
	a.fullTree.AddFast(ch)

	err = a.thread.AddChange(&thread.RawChange{
		Payload:   marshalled,
		Signature: signature,
		Id:        changeContent.Id,
	})
	if err != nil {
		return nil, err
	}

	a.thread.SetHeads([]string{ch.Id})
	return a.fullTree.attached[changeContent.Id], nil
}

func (a *aclTree) AddChanges(changes ...*Change) (AddResult, error) {
	var aclChanges []*Change
	for _, ch := range changes {
		if ch.IsACLChange() {
			aclChanges = append(aclChanges, ch)
			break
		}
	}

	// TODO: understand the common snapshot problem
	prevHeads := a.fullTree.Heads()
	prevRoot := a.
	mode := a.fullTree.Add(changes...)
	switch mode {
	case acltree.Nothing:
		return d.docContext.docState, UpdateResultNoAction, nil
	case acltree.Rebuild:
		res, err := d.Build()
		return res, UpdateResultRebuild, err
	default:
		break
	}
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

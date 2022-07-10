package acltree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/aclchanges/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/thread"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
	"github.com/gogo/protobuf/proto"
	"sync"
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

// TODO: Change add change content to include ACLChangeBuilder
type ACLTree interface {
	ACLState() *ACLState
	AddContent(changeContent *ChangeContent) (*Change, error)
	AddChanges(changes ...*Change) (AddResult, error)
	Heads() []string
	Iterate(func(change *Change) bool)
	IterateFrom(string, func(change *Change) bool)
	HasChange(string) bool
}

type aclTree struct {
	thread      thread.Thread
	accountData *account.AccountData

	fullTree *Tree
	aclTree  *Tree // TODO: right now we don't use it, we can probably have only local var for now. This tree is built from start of the document
	aclState *ACLState

	treeBuilder       *treeBuilder
	aclTreeBuilder    *aclTreeBuilder
	aclStateBuilder   *aclStateBuilder
	snapshotValidator *snapshotValidator

	sync.Mutex
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

// TODO: this is not used for now, in future we should think about not making full tree rebuild
func (a *aclTree) rebuildFromTree(validateSnapshot bool) (err error) {
	if validateSnapshot {
		err = a.snapshotValidator.Init(a.aclTree)
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
	a.aclTree, err = a.aclTreeBuilder.Build()
	if err != nil {
		return err
	}

	if !fromStart {
		err = a.snapshotValidator.Init(a.aclTree)
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
	a.Lock()
	defer a.Unlock()
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

	err = a.thread.AddRawChange(&thread.RawChange{
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

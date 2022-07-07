package data

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/data/threadmodels"
)

type AccountData struct {
	Identity string
	EncKey   threadmodels.EncryptionPrivKey
}

type Document struct {
	thread        threadmodels.Thread
	stateProvider InitialStateProvider
	accountData   *AccountData
	decoder       threadmodels.SigningPubKeyDecoder

	treeBuilder       *TreeBuilder
	aclTreeBuilder    *ACLTreeBuilder
	aclStateBuilder   *ACLStateBuilder
	snapshotValidator *SnapshotValidator
	docStateBuilder   *documentStateBuilder

	docContext *documentContext
}

type UpdateResult int

const (
	UpdateResultNoAction UpdateResult = iota
	UpdateResultAppend
	UpdateResultRebuild
)

func NewDocument(
	thread threadmodels.Thread,
	stateProvider InitialStateProvider,
	accountData *AccountData) *Document {

	decoder := threadmodels.NewEd25519Decoder()
	return &Document{
		thread:            thread,
		stateProvider:     stateProvider,
		accountData:       accountData,
		decoder:           decoder,
		aclTreeBuilder:    NewACLTreeBuilder(thread, decoder),
		treeBuilder:       NewTreeBuilder(thread, decoder),
		snapshotValidator: NewSnapshotValidator(decoder, accountData),
		aclStateBuilder:   NewACLStateBuilder(decoder, accountData),
		docStateBuilder:   newDocumentStateBuilder(stateProvider),
		docContext:        &documentContext{},
	}
}

func (d *Document) Update(changes ...*threadmodels.RawChange) (DocumentState, UpdateResult, error) {
	var treeChanges []*Change

	var foundACLChange bool
	for _, ch := range changes {
		aclChange, err := d.treeBuilder.makeVerifiedACLChange(ch)
		if err != nil {
			return nil, UpdateResultNoAction, fmt.Errorf("change with id %s is incorrect: %w", ch.Id, err)
		}

		treeChange := d.treeBuilder.changeCreator(ch.Id, aclChange)
		treeChanges = append(treeChanges, treeChange)

		// this already sets MaybeHeads to include new changes
		// TODO: change this behaviour as non-obvious, because it is not evident from the interface
		err = d.thread.AddChange(ch)
		if err != nil {
			return nil, UpdateResultNoAction, fmt.Errorf("change with id %s cannot be added: %w", ch.Id, err)
		}
		if treeChange.IsACLChange() {
			foundACLChange = true
		}
	}

	if foundACLChange {
		res, err := d.Build()
		return res, UpdateResultRebuild, err
	}

	prevHeads := d.docContext.fullTree.Heads()
	mode := d.docContext.fullTree.Add(treeChanges...)
	switch mode {
	case Nothing:
		return d.docContext.docState, UpdateResultNoAction, nil
	case Rebuild:
		res, err := d.Build()
		return res, UpdateResultRebuild, err
	default:
		break
	}

	// decrypting everything, because we have no new keys
	for _, ch := range treeChanges {
		if ch.Content.GetChangesData() != nil {
			key, exists := d.docContext.aclState.userReadKeys[ch.Content.CurrentReadKeyHash]
			if !exists {
				err := fmt.Errorf("failed to find key with hash: %d", ch.Content.CurrentReadKeyHash)
				return nil, UpdateResultNoAction, err
			}

			err := ch.DecryptContents(key)
			if err != nil {
				err = fmt.Errorf("failed to decrypt contents for hash: %d", ch.Content.CurrentReadKeyHash)
				return nil, UpdateResultNoAction, err
			}
		}
	}

	// because for every new change we know it was after any of the previous heads
	// each of previous heads must have same "Next" nodes
	// so it doesn't matter which one we choose
	// so we choose first one
	newState, err := d.docStateBuilder.appendFrom(prevHeads[0], d.docContext.docState)
	if err != nil {
		res, _ := d.Build()
		return res, UpdateResultRebuild, fmt.Errorf("could not add changes to state, rebuilded")
	}

	// setting all heads
	d.thread.SetHeads(d.docContext.fullTree.Heads())
	// this should be the entrypoint when we build the document
	d.thread.SetMaybeHeads(d.docContext.fullTree.Heads())

	return newState, UpdateResultAppend, nil
}

func (d *Document) Build() (DocumentState, error) {
	return d.build(false)
}

func (d *Document) build(fromStart bool) (DocumentState, error) {
	d.treeBuilder.Init()
	d.aclTreeBuilder.Init()

	var err error
	d.docContext.fullTree, err = d.treeBuilder.Build(fromStart)
	if err != nil {
		return nil, err
	}

	d.docContext.aclTree, err = d.aclTreeBuilder.Build()
	if err != nil {
		return nil, err
	}

	if !fromStart {
		err = d.snapshotValidator.Init(d.docContext.aclTree)
		if err != nil {
			return nil, err
		}

		valid, err := d.snapshotValidator.ValidateSnapshot(d.docContext.fullTree.root)
		if err != nil {
			return nil, err
		}
		if !valid {
			return d.build(true)
		}
	}
	err = d.aclStateBuilder.Init(d.docContext.fullTree)
	if err != nil {
		return nil, err
	}

	d.docContext.aclState, err = d.aclStateBuilder.Build()
	if err != nil {
		return nil, err
	}

	d.docStateBuilder.init(d.docContext.aclState, d.docContext.fullTree)
	d.docContext.docState, err = d.docStateBuilder.build()
	if err != nil {
		return nil, err
	}

	// setting all heads
	d.thread.SetHeads(d.docContext.fullTree.Heads())
	// this should be the entrypoint when we build the document
	d.thread.SetMaybeHeads(d.docContext.fullTree.Heads())

	return d.docContext.docState, nil
}

func (d *Document) State() DocumentState {
	return nil
}

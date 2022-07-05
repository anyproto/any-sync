package data

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/data/pb"
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
	UpdateResultAppend = iota
	UpdateResultRebuild
	UpdateResultExists
	UpdateResultNoAction
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

func (d *Document) Update(changes []*pb.ACLChange) (DocumentState, UpdateResult, error) {
	return nil, 0, nil
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
		d.snapshotValidator.Init(d.docContext.aclTree)
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

	return d.docContext.docState, nil
}

func (d *Document) State() DocumentState {
	return nil
}

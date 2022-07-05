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
	thread          threadmodels.Thread
	stateProvider   InitialStateProvider
	accountData     AccountData
	decoder         threadmodels.SigningPubKeyDecoder
	aclTree         *Tree
	fullTree        *Tree
	aclTreeBuilder  *ACLTreeBuilder
	aclStateBuilder *ACLStateBuilder
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
	accountData AccountData) *Document {
	return &Document{
		thread:        thread,
		stateProvider: stateProvider,
		accountData:   accountData,
		decoder:       threadmodels.NewEd25519Decoder(),
	}
}

func (d *Document) Update(changes []*pb.ACLChange) (DocumentState, UpdateResult, error) {
	return nil, 0, nil
}

func (d *Document) Build() (DocumentState, error) {
	treeBuilder := NewTreeBuilder(d.thread, threadmodels.NewEd25519Decoder())

	return treeBuilder.Build(fromStart)
	return nil, nil
}

func (d *Document) State() DocumentState {
	return nil
}

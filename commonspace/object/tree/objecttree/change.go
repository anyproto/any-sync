package objecttree

import (
	"errors"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/util/crypto"
)

var (
	ErrIncorrectSignature = errors.New("change has incorrect signature")
	ErrIncorrectCid       = errors.New("change has incorrect CID")
)

// Change is an abstract type for all types of changes
type Change struct {
	Next        []*Change
	PreviousIds []string
	Previous    []*Change
	AclHeadId   string
	Id          string
	SnapshotId  string
	Timestamp   int64
	ReadKeyId   string
	Identity    crypto.PubKey
	Data        []byte
	// TODO: add call one time comment
	Model      interface{}
	Signature  []byte
	DataType   string
	IsSnapshot bool
	IsDerived  bool
	IsNew      bool
	OrderId    string

	// SnapshotCounter is the number of previous snapshots in the current branch. For the first snapshot it's zero, for all next changes
	// it increases every time a new snapshot is created. You can think of it as depth of the node in a snapshots tree.
	SnapshotCounter int

	// using this on build stage
	rawChange *treechangeproto.RawTreeChangeWithId

	// iterator helpers
	visited          bool
	branchesFinished bool
	lastSeenOrderId  string
}

func NewChangeFromRoot(id string, identity crypto.PubKey, ch *treechangeproto.RootChange, signature []byte, isDerived bool) *Change {
	changeInfo := &treechangeproto.TreeChangeInfo{
		ChangeType:    ch.ChangeType,
		ChangePayload: ch.ChangePayload,
	}
	data, _ := changeInfo.MarshalVT()
	return &Change{
		Next:       nil,
		AclHeadId:  ch.AclHeadId,
		Id:         id,
		IsSnapshot: true,
		Timestamp:  ch.Timestamp,
		Identity:   identity,
		Signature:  signature,
		Data:       data,
		Model:      changeInfo,
		IsDerived:  isDerived,
	}
}

func (ch *Change) Cid() string {
	return ch.Id
}

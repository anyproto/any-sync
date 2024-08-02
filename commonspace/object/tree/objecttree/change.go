package objecttree

import (
	"errors"

	"github.com/anyproto/protobuf/proto"

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
	AclHeadId   string
	Id          string
	SnapshotId  string
	Timestamp   int64
	ReadKeyId   string
	Identity    crypto.PubKey
	Data        []byte
	Model       interface{}
	Signature   []byte
	DataType    string
	IsSnapshot  bool
	IsDerived   bool

	// iterator helpers
	visited          bool
	branchesFinished bool
}

func NewChangeFromRoot(id string, identity crypto.PubKey, ch *treechangeproto.RootChange, signature []byte, isDerived bool) *Change {
	changeInfo := &treechangeproto.TreeChangeInfo{
		ChangeType:    ch.ChangeType,
		ChangePayload: ch.ChangePayload,
	}
	data, _ := proto.Marshal(changeInfo)
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

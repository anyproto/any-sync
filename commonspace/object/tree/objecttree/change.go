package objecttree

import (
	"errors"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
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
	IsSnapshot  bool
	Timestamp   int64
	ReadKeyHash uint64
	Identity    string
	Data        []byte
	Model       interface{}

	// iterator helpers
	visited          bool
	branchesFinished bool

	Signature []byte
}

func NewChange(id string, ch *treechangeproto.TreeChange, signature []byte) *Change {
	return &Change{
		Next:        nil,
		PreviousIds: ch.TreeHeadIds,
		AclHeadId:   ch.AclHeadId,
		Timestamp:   ch.Timestamp,
		ReadKeyHash: ch.CurrentReadKeyHash,
		Id:          id,
		Data:        ch.ChangesData,
		SnapshotId:  ch.SnapshotBaseId,
		IsSnapshot:  ch.IsSnapshot,
		Identity:    string(ch.Identity),
		Signature:   signature,
	}
}

func NewChangeFromRoot(id string, ch *treechangeproto.RootChange, signature []byte) *Change {
	return &Change{
		Next:       nil,
		AclHeadId:  ch.AclHeadId,
		Id:         id,
		IsSnapshot: true,
		Identity:   string(ch.Identity),
		Signature:  signature,
		Data:       []byte(ch.ChangeType),
	}
}

func (ch *Change) Cid() string {
	return ch.Id
}

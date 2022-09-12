package tree

import (
	"errors"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/gogo/protobuf/proto"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/symmetric"
)

var (
	ErrIncorrectSignature = errors.New("change has incorrect signature")
	ErrIncorrectCID       = errors.New("change has incorrect CID")
)

type ChangeContent struct {
	ChangesData proto.Marshaler
	ACLData     *aclpb.ACLData
	Id          string // TODO: this is just for testing, because id should be created automatically from content
}

// Change is an abstract type for all types of changes
type Change struct {
	Next            []*Change
	PreviousIds     []string
	Id              string
	SnapshotId      string
	IsSnapshot      bool
	DecryptedChange []byte      // TODO: check if we need it
	ParsedModel     interface{} // TODO: check if we need it

	// iterator helpers
	visited          bool
	branchesFinished bool

	Content  *aclpb.TreeChange
	Identity string
	Sign     []byte
}

func (ch *Change) ProtoChange() proto.Marshaler {
	return ch.Content
}

func (ch *Change) DecryptContents(key *symmetric.Key) error {
	// if the document is already decrypted
	if ch.Content.CurrentReadKeyHash == 0 {
		return nil
	}
	decrypted, err := key.Decrypt(ch.Content.ChangesData)
	if err != nil {
		return fmt.Errorf("failed to decrypt changes data: %w", err)
	}

	ch.DecryptedChange = decrypted
	return nil
}

func NewChange(id string, ch *aclpb.TreeChange, signature []byte) *Change {
	return &Change{
		Next:        nil,
		PreviousIds: ch.TreeHeadIds,
		Id:          id,
		Content:     ch,
		SnapshotId:  ch.SnapshotBaseId,
		IsSnapshot:  ch.IsSnapshot,
		Identity:    string(ch.Identity),
		Sign:        signature,
	}
}

func (ch *Change) DecryptedChangeContent() []byte {
	return ch.DecryptedChange
}

func (ch *Change) Signature() []byte {
	return ch.Sign
}

func (ch *Change) CID() string {
	return ch.Id
}

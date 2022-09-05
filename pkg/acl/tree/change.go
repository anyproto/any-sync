package tree

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/gogo/protobuf/proto"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/symmetric"
)

type ChangeContent struct {
	ChangesData proto.Marshaler
	ACLData     *aclpb.ACLChangeACLData
	Id          string // TODO: this is just for testing, because id should be created automatically from content
}

// Change is an abstract type for all types of changes
type Change struct {
	Next            []*Change
	PreviousIds     []string
	Id              string
	SnapshotId      string
	IsSnapshot      bool
	DecryptedChange []byte // TODO: check if we need it
	ParsedModel     interface{}

	// iterator helpers
	visited          bool
	branchesFinished bool

	Content *aclpb.Change
	Sign    []byte
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

func NewChangeFromRaw(rawChange *aclpb.RawChange) (*Change, error) {
	unmarshalled := &aclpb.Change{}
	err := proto.Unmarshal(rawChange.Payload, unmarshalled)
	if err != nil {
		return nil, err
	}

	ch := NewChange(rawChange.Id, unmarshalled)
	ch.Sign = rawChange.Signature
	return ch, nil
}

func NewVerifiedChangeFromRaw(
	rawChange *aclpb.RawChange,
	kch *keychain) (*Change, error) {
	unmarshalled := &aclpb.Change{}
	err := proto.Unmarshal(rawChange.Payload, unmarshalled)
	if err != nil {
		return nil, err
	}

	identityKey, err := kch.getOrAdd(unmarshalled.Identity)
	if err != nil {
		return nil, err
	}

	res, err := identityKey.Verify(rawChange.Payload, rawChange.Signature)
	if err != nil {
		return nil, err
	}
	if !res {
		return nil, fmt.Errorf("change has incorrect signature")
	}

	return NewChange(rawChange.Id, unmarshalled), nil
}

func NewChange(id string, ch *aclpb.Change) *Change {
	return &Change{
		Next:        nil,
		PreviousIds: ch.TreeHeadIds,
		Id:          id,
		Content:     ch,
		SnapshotId:  ch.SnapshotBaseId,
		IsSnapshot:  ch.IsSnapshot,
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

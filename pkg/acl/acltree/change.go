package acltree

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/gogo/protobuf/proto"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/symmetric"
)

type ChangeContent struct {
	ChangesData proto.Marshaler
	ACLData     *pb.ACLChangeACLData
	Id          string // TODO: this is just for testing, because id should be created automatically from content
}

// Change is an abstract type for all types of changes
type Change struct {
	Next                    []*Change
	Unattached              []*Change
	PreviousIds             []string
	Id                      string
	SnapshotId              string
	IsSnapshot              bool
	DecryptedDocumentChange []byte

	Content *pb.ACLChange
	Sign    []byte
}

func (ch *Change) DecryptContents(key *symmetric.Key) error {
	if ch.Content.ChangesData == nil {
		return nil
	}

	decrypted, err := key.Decrypt(ch.Content.ChangesData)
	if err != nil {
		return fmt.Errorf("failed to decrypt changes data: %w", err)
	}

	ch.DecryptedDocumentChange = decrypted
	return nil
}

func (ch *Change) IsACLChange() bool {
	return ch.Content.GetAclData() != nil
}

func NewFromRawChange(rawChange *treestorage.RawChange) (*Change, error) {
	unmarshalled := &pb.ACLChange{}
	err := proto.Unmarshal(rawChange.Payload, unmarshalled)
	if err != nil {
		return nil, err
	}

	ch := NewChange(rawChange.Id, unmarshalled)
	ch.Sign = rawChange.Signature
	return ch, nil
}

func NewChange(id string, ch *pb.ACLChange) *Change {
	return &Change{
		Next:        nil,
		PreviousIds: ch.TreeHeadIds,
		Id:          id,
		Content:     ch,
		SnapshotId:  ch.SnapshotBaseId,
		IsSnapshot:  ch.GetAclData().GetAclSnapshot() != nil,
	}
}

func NewACLChange(id string, ch *pb.ACLChange) *Change {
	return &Change{
		Next:        nil,
		PreviousIds: ch.AclHeadIds,
		Id:          id,
		Content:     ch,
		SnapshotId:  ch.SnapshotBaseId,
		IsSnapshot:  ch.GetAclData().GetAclSnapshot() != nil,
	}
}

func (ch *Change) ProtoChange() *pb.ACLChange {
	return ch.Content
}

func (ch *Change) DecryptedChangeContent() []byte {
	return ch.DecryptedDocumentChange
}

func (ch *Change) Signature() []byte {
	return ch.Sign
}

func (ch *Change) CID() string {
	return ch.Id
}

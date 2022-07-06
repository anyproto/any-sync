package data

import (
	"fmt"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/data/pb"
	"github.com/textileio/go-threads/crypto/symmetric"
)

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

package data

import (
	"fmt"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/pb"
	"github.com/gogo/protobuf/proto"
	"github.com/textileio/go-threads/crypto/symmetric"
)

// Change is an abstract type for all types of changes
type Change struct {
	Next                    []*Change
	Unattached              []*Change
	PreviousIds             []string
	Id                      string
	SnapshotId              string
	LogHeads                map[string]string
	IsSnapshot              bool
	DecryptedDocumentChange *pb.ACLChangeChangeData

	Content *pb.ACLChange
}

func (ch *Change) DecryptContents(key *symmetric.Key) error {
	if ch.Content.ChangesData == nil {
		return nil
	}

	var changesData pb.ACLChangeChangeData
	decrypted, err := key.Decrypt(ch.Content.ChangesData)
	if err != nil {
		return fmt.Errorf("failed to decrypt changes data: %w", err)
	}

	err = proto.Unmarshal(decrypted, &changesData)
	if err != nil {
		return fmt.Errorf("failed to umarshall into ChangesData: %w", err)
	}
	ch.DecryptedDocumentChange = &changesData
	return nil
}

func (ch *Change) IsACLChange() bool {
	return ch.Content.GetAclData() != nil
}

func NewChange(id string, ch *pb.ACLChange) (*Change, error) {
	return &Change{
		Next:        nil,
		PreviousIds: ch.TreeHeadIds,
		Id:          id,
		Content:     ch,
		SnapshotId:  ch.SnapshotBaseId,
		IsSnapshot:  ch.GetAclData().GetAclSnapshot() != nil,
		LogHeads:    ch.GetLogHeads(),
	}, nil
}

func NewACLChange(id string, ch *pb.ACLChange) (*Change, error) {
	return &Change{
		Next:        nil,
		PreviousIds: ch.AclHeadIds,
		Id:          id,
		Content:     ch,
		SnapshotId:  ch.SnapshotBaseId,
		IsSnapshot:  ch.GetAclData().GetAclSnapshot() != nil,
		LogHeads:    ch.GetLogHeads(),
	}, nil
}

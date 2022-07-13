package thread

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/acl/aclchanges"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/acl/thread/pb"
)

// TODO: change methods to have errors as a return parameter, because we will be dealing with a real database
type Thread interface {
	ID() string

	Header() *pb.ThreadHeader
	Heads() []string
	Orphans() []string
	SetHeads(heads []string)
	RemoveOrphans(orphan ...string)
	AddOrphans(orphan ...string)

	AddRawChange(change *RawChange) error
	AddChange(change aclchanges.Change) error

	// TODO: have methods with raw changes also
	GetChange(ctx context.Context, recordID string) (*RawChange, error)
}

type RawChange struct {
	Payload   []byte
	Signature []byte
	Id        string
}

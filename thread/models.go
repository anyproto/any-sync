package thread

import (
	"context"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/aclchanges"
)

// TODO: change methods to have errors as a return parameter, because we will be dealing with a real database
type Thread interface {
	ID() string

	Heads() []string
	PossibleHeads() []string
	SetHeads(heads []string)
	SetPossibleHeads(heads []string)
	AddPossibleHead(head string)

	AddRawChange(change *RawChange) error
	AddChange(change aclchanges.Change) error
	GetChange(ctx context.Context, recordID string) (*RawChange, error)
}

type RawChange struct {
	Payload   []byte
	Signature []byte
	Id        string
}

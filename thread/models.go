package thread

import (
	"context"
)

type Thread interface {
	ID() string
	Heads() []string
	MaybeHeads() []string
	GetChange(ctx context.Context, recordID string) (*RawChange, error)
	SetHeads(heads []string)
	SetMaybeHeads(heads []string)
	AddChange(change *RawChange) error
}

type RawChange struct {
	Payload   []byte
	Signature []byte
	Id        string
}

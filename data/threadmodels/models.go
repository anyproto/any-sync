package threadmodels

import (
	"context"
	"github.com/gogo/protobuf/proto"
)

type Thread interface {
	ID() string
	Heads() []string
	// TODO: add ACL heads
	GetChange(ctx context.Context, recordID string) (*RawChange, error)
	PushChange(payload proto.Marshaler) (id string, err error)
}

type RawChange struct {
	Payload   []byte
	Signature []byte
	Id        string
}

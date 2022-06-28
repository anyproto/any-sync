package threadmodels

import (
	"context"

	"github.com/gogo/protobuf/proto"
)

type Thread interface {
	ID() string
	GetLogs() ([]ThreadLog, error)
	GetRecord(ctx context.Context, recordID string) (*ThreadRecord, error)
	PushRecord(payload proto.Marshaler) (id string, err error)

	// SubscribeForRecords()
}

type SignedPayload struct {
	Payload   []byte
	Signature []byte
}

type ThreadRecord struct {
	PrevId string
	Id     string
	LogId  string
	Signed *SignedPayload
}

type ThreadLog struct {
	ID      string
	Head    string
	Counter int64
}

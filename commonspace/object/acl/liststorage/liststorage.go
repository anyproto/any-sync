//go:generate mockgen -destination mock_liststorage/mock_liststorage.go github.com/anyproto/any-sync/commonspace/object/acl/liststorage ListStorage
package liststorage

import (
	"context"
	"errors"

	"github.com/anyproto/any-sync/consensus/consensusproto"
)

var (
	ErrUnknownAclId  = errors.New("acl does not exist")
	ErrAclExists     = errors.New("acl already exists")
	ErrUnknownRecord = errors.New("record doesn't exist")
)

type Exporter interface {
	ListStorage(root *consensusproto.RawRecordWithId) (ListStorage, error)
}

type ListStorage interface {
	Id() string
	Root() (*consensusproto.RawRecordWithId, error)
	Head() (string, error)
	SetHead(headId string) error

	GetRawRecord(ctx context.Context, id string) (*consensusproto.RawRecordWithId, error)
	AddRawRecord(ctx context.Context, rec *consensusproto.RawRecordWithId) error
}

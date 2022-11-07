//go:generate mockgen -destination mock_storage/mock_storage.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage ListStorage,TreeStorage
package storage

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/aclrecordproto"
)

var ErrUnknownACLId = errors.New("acl does not exist")
var ErrACLExists = errors.New("acl already exists")
var ErrUnknownRecord = errors.New("record doesn't exist")

type ListStorage interface {
	Id() string
	Root() (*aclrecordproto.RawACLRecordWithId, error)
	Head() (string, error)
	SetHead(headId string) error

	GetRawRecord(ctx context.Context, id string) (*aclrecordproto.RawACLRecordWithId, error)
	AddRawRecord(ctx context.Context, rec *aclrecordproto.RawACLRecordWithId) error
}

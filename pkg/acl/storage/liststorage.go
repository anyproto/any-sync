//go:generate mockgen -destination mock_storage/mock_storage.go github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage ListStorage,TreeStorage
package storage

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclrecordproto"
)

var ErrUnknownACLId = errors.New("acl does not exist")
var ErrACLExists = errors.New("acl already exists")
var ErrUnknownRecord = errors.New("record doesn't exist")

type ListStorage interface {
	Storage
	Root() (*aclrecordproto.RawACLRecordWithId, error)
	Head() (string, error)

	GetRawRecord(ctx context.Context, id string) (*aclrecordproto.RawACLRecordWithId, error)
	AddRawRecord(ctx context.Context, rec *aclrecordproto.RawACLRecordWithId) error
}

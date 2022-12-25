//go:generate mockgen -destination mock_liststorage/mock_liststorage.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/liststorage ListStorage
package liststorage

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/aclrecordproto"
)

var (
	ErrUnknownACLId  = errors.New("acl does not exist")
	ErrACLExists     = errors.New("acl already exists")
	ErrUnknownRecord = errors.New("record doesn't exist")
)

type ListStorage interface {
	Id() string
	Root() (*aclrecordproto.RawACLRecordWithId, error)
	Head() (string, error)
	SetHead(headId string) error

	GetRawRecord(ctx context.Context, id string) (*aclrecordproto.RawACLRecordWithId, error)
	AddRawRecord(ctx context.Context, rec *aclrecordproto.RawACLRecordWithId) error
}

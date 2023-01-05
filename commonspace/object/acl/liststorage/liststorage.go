//go:generate mockgen -destination mock_liststorage/mock_liststorage.go github.com/anytypeio/any-sync/commonspace/object/acl/liststorage ListStorage
package liststorage

import (
	"context"
	"errors"
	"github.com/anytypeio/any-sync/commonspace/object/acl/aclrecordproto"
)

var (
	ErrUnknownAclId  = errors.New("acl does not exist")
	ErrAclExists     = errors.New("acl already exists")
	ErrUnknownRecord = errors.New("record doesn't exist")
)

type ListStorage interface {
	Id() string
	Root() (*aclrecordproto.RawAclRecordWithId, error)
	Head() (string, error)
	SetHead(headId string) error

	GetRawRecord(ctx context.Context, id string) (*aclrecordproto.RawAclRecordWithId, error)
	AddRawRecord(ctx context.Context, rec *aclrecordproto.RawAclRecordWithId) error
}

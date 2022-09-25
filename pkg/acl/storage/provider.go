package storage

import (
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treechangeproto"
)

var ErrUnknownTreeId = errors.New("tree does not exist")

type TreeStorageCreatePayload struct {
	TreeId        string
	RootRawChange *treechangeproto.RawTreeChangeWithId
	Changes       []*treechangeproto.RawTreeChangeWithId
	Heads         []string
}

type ACLListStorageCreatePayload struct {
	ListId  string
	Records []*aclrecordproto.RawACLRecordWithId
}

type Provider interface {
	Storage(id string) (Storage, error)
	CreateTreeStorage(payload TreeStorageCreatePayload) (TreeStorage, error)
	CreateACLListStorage(payload ACLListStorageCreatePayload) (ListStorage, error)
}

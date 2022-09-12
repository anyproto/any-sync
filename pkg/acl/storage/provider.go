package storage

import (
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
)

var ErrUnknownTreeId = errors.New("tree does not exist")

type TreeStorageCreatePayload struct {
	TreeId  string
	Header  *aclpb.TreeHeader
	Changes []*aclpb.RawTreeChangeWithId
	Heads   []string
}

type ACLListStorageCreatePayload struct {
	ListId  string
	Header  *aclpb.ACLHeader
	Records []*aclpb.RawACLRecord
}

type Provider interface {
	Storage(id string) (Storage, error)
	AddStorage(id string, st Storage) error
	CreateTreeStorage(payload TreeStorageCreatePayload) (TreeStorage, error)
	CreateACLListStorage(payload ACLListStorageCreatePayload) (ListStorage, error)
}

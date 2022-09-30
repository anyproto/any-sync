//go:generate mockgen -destination mock_storage/mock_storage.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage SpaceStorageProvider,SpaceStorage
package storage

import (
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
)

const CName = "commonspace.storage"

var ErrSpaceStorageExists = errors.New("space storage exists")

type SpaceStorage interface {
	storage.Provider
	ACLStorage() (storage.ListStorage, error)
	SpaceHeader() (*spacesyncproto.SpaceHeader, error)
	StoredIds() ([]string, error)
}

type SpaceStorageCreatePayload struct {
	RecWithId   *aclrecordproto.RawACLRecordWithId
	SpaceHeader *spacesyncproto.SpaceHeader
	Id          string
}

type SpaceStorageProvider interface {
	app.Component
	SpaceStorage(id string) (SpaceStorage, error)
	CreateSpaceStorage(payload SpaceStorageCreatePayload) (SpaceStorage, error)
}

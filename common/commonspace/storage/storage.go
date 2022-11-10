//go:generate mockgen -destination mock_storage/mock_storage.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage SpaceStorageProvider,SpaceStorage
package storage

import (
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
)

const CName = "commonspace.storage"

var ErrSpaceStorageExists = errors.New("space storage exists")
var ErrSpaceStorageMissing = errors.New("space storage missing")

type SpaceStorage interface {
	storage.Provider
	Id() string
	SpaceSettingsId() string
	ACLStorage() (storage.ListStorage, error)
	SpaceHeader() (*spacesyncproto.RawSpaceHeaderWithId, error)
	StoredIds() ([]string, error)
	Close() error
}

type SpaceStorageCreatePayload struct {
	AclWithId           *aclrecordproto.RawACLRecordWithId
	SpaceHeaderWithId   *spacesyncproto.RawSpaceHeaderWithId
	SpaceSettingsWithId *treechangeproto.RawTreeChangeWithId
}

type SpaceStorageProvider interface {
	app.Component
	SpaceStorage(id string) (SpaceStorage, error)
	CreateSpaceStorage(payload SpaceStorageCreatePayload) (SpaceStorage, error)
}

func ValidateSpaceStorageCreatePayload(payload SpaceStorageCreatePayload) (err error) {
	// TODO: add proper validation
	return nil
}

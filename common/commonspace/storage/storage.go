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

var (
	ErrSpaceStorageExists  = errors.New("space storage exists")
	ErrSpaceStorageMissing = errors.New("space storage missing")

	ErrTreeStorageAlreadyDeleted = errors.New("tree storage already deleted")
)

const (
	TreeDeletedStatusQueued  = "queued"
	TreeDeletedStatusDeleted = "deleted"
)

type SpaceStorage interface {
	storage.Provider
	Id() string
	SetTreeDeletedStatus(id, state string) error
	TreeDeletedStatus(id string) (string, error)
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

//go:generate mockgen -destination mock_storage/mock_storage.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacestorage SpaceStorageProvider,SpaceStorage
package spacestorage

import (
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/liststorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
)

const CName = "common.commonspace.spacestorage"

var (
	ErrSpaceStorageExists  = errors.New("space storage exists")
	ErrSpaceStorageMissing = errors.New("space storage missing")

	ErrTreeStorageAlreadyDeleted = errors.New("tree storage already deleted")
)

const (
	TreeDeletedStatusQueued  = "queued"
	TreeDeletedStatusDeleted = "deleted"
)

// TODO: consider moving to some file with all common interfaces etc
type SpaceStorage interface {
	treestorage.Provider
	Id() string
	SetTreeDeletedStatus(id, state string) error
	TreeDeletedStatus(id string) (string, error)
	SpaceSettingsId() string
	ACLStorage() (liststorage.ListStorage, error)
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

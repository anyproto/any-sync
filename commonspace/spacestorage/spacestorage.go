//go:generate mockgen -destination mock_spacestorage/mock_spacestorage.go github.com/anytypeio/any-sync/commonspace/spacestorage SpaceStorage
package spacestorage

import (
	"context"
	"errors"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/any-sync/commonspace/object/acl/liststorage"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
)

const CName = "common.commonspace.spacestorage"

var (
	ErrSpaceStorageExists   = errors.New("space storage exists")
	ErrSpaceStorageMissing  = errors.New("space storage missing")
	ErrIncorrectSpaceHeader = errors.New("incorrect space header")

	ErrTreeStorageAlreadyDeleted = errors.New("tree storage already deleted")
)

const (
	TreeDeletedStatusQueued  = "queued"
	TreeDeletedStatusDeleted = "deleted"
)

// TODO: consider moving to some file with all common interfaces etc
type SpaceStorage interface {
	Id() string
	SetSpaceDeleted() error
	IsSpaceDeleted() (bool, error)
	SetTreeDeletedStatus(id, state string) error
	TreeDeletedStatus(id string) (string, error)
	SpaceSettingsId() string
	AclStorage() (liststorage.ListStorage, error)
	SpaceHeader() (*spacesyncproto.RawSpaceHeaderWithId, error)
	StoredIds() ([]string, error)
	TreeRoot(id string) (*treechangeproto.RawTreeChangeWithId, error)
	TreeStorage(id string) (treestorage.TreeStorage, error)
	HasTree(id string) (bool, error)
	CreateTreeStorage(payload treestorage.TreeStorageCreatePayload) (treestorage.TreeStorage, error)
	WriteSpaceHash(hash string) error
	ReadSpaceHash() (hash string, err error)

	Close() error
}

type SpaceStorageCreatePayload struct {
	AclWithId           *aclrecordproto.RawAclRecordWithId
	SpaceHeaderWithId   *spacesyncproto.RawSpaceHeaderWithId
	SpaceSettingsWithId *treechangeproto.RawTreeChangeWithId
}

type SpaceStorageProvider interface {
	app.Component
	WaitSpaceStorage(ctx context.Context, id string) (SpaceStorage, error)
	SpaceExists(id string) bool
	CreateSpaceStorage(payload SpaceStorageCreatePayload) (SpaceStorage, error)
}

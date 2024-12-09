//go:generate mockgen -destination mock_spacestorage/mock_spacestorage.go github.com/anyproto/any-sync/commonspace/spacestorage SpaceStorage
package spacestorage

import (
	"context"
	"errors"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/headsync/statestorage"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

const CName = "common.commonspace.spacestorage"

var (
	ErrSpaceStorageExists   = errors.New("space storage exists")
	ErrSpaceStorageMissing  = errors.New("space storage missing")
	ErrIncorrectSpaceHeader = errors.New("incorrect space header")

	ErrTreeStorageAlreadyDeleted = errors.New("tree storage already deleted")
)

type SpaceStorage interface {
	app.ComponentRunnable
	Id() string
	HeadStorage() headstorage.HeadStorage
	StateStorage() statestorage.StateStorage
	AclStorage() (list.Storage, error)
	TreeStorage(ctx context.Context, id string) (objecttree.Storage, error)
	CreateTreeStorage(ctx context.Context, payload treestorage.TreeStorageCreatePayload) (objecttree.Storage, error)
}

type SpaceStorageCreatePayload struct {
	AclWithId           *consensusproto.RawRecordWithId
	SpaceHeaderWithId   *spacesyncproto.RawSpaceHeaderWithId
	SpaceSettingsWithId *treechangeproto.RawTreeChangeWithId
}

type SpaceStorageProvider interface {
	app.Component
	WaitSpaceStorage(ctx context.Context, id string) (SpaceStorage, error)
	SpaceExists(id string) bool
	CreateSpaceStorage(payload SpaceStorageCreatePayload) (SpaceStorage, error)
}

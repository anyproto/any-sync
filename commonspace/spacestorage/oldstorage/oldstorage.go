package oldstorage

import (
	"context"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

const (
	TreeDeletedStatusQueued  = "queued"
	TreeDeletedStatusDeleted = "deleted"
)

type TreeStorage interface {
	Id() string
	Root() (*treechangeproto.RawTreeChangeWithId, error)
	Heads() ([]string, error)
	SetHeads(heads []string) error
	AddRawChange(change *treechangeproto.RawTreeChangeWithId) error
	AddRawChangesSetHeads(changes []*treechangeproto.RawTreeChangeWithId, heads []string) error
	GetAllChangeIds() ([]string, error)
	GetAllChanges() ([]*treechangeproto.RawTreeChangeWithId, error)
	IterateChanges(proc func(id string, rawChange []byte) error) error

	GetRawChange(ctx context.Context, id string) (*treechangeproto.RawTreeChangeWithId, error)
	GetAppendRawChange(ctx context.Context, buf []byte, id string) (*treechangeproto.RawTreeChangeWithId, error)
	HasChange(ctx context.Context, id string) (bool, error)
	Delete() error
}

type ListStorage interface {
	Id() string
	Root() (*consensusproto.RawRecordWithId, error)
	Head() (string, error)
	SetHead(headId string) error

	GetRawRecord(ctx context.Context, id string) (*consensusproto.RawRecordWithId, error)
	AddRawRecord(ctx context.Context, rec *consensusproto.RawRecordWithId) error
}

type SpaceStorage interface {
	app.ComponentRunnable
	Id() string
	SetSpaceDeleted() error
	IsSpaceDeleted() (bool, error)
	SetTreeDeletedStatus(id, state string) error
	TreeDeletedStatus(id string) (string, error)
	SpaceSettingsId() string
	AclStorage() (ListStorage, error)
	SpaceHeader() (*spacesyncproto.RawSpaceHeaderWithId, error)
	StoredIds() ([]string, error)
	TreeRoot(id string) (*treechangeproto.RawTreeChangeWithId, error)
	TreeStorage(id string) (TreeStorage, error)
	HasTree(id string) (bool, error)
	CreateTreeStorage(payload treestorage.TreeStorageCreatePayload) (TreeStorage, error)
	WriteSpaceHash(hash string) error
	ReadSpaceHash() (hash string, err error)
}

type SpaceStorageProvider interface {
	app.Component
	WaitSpaceStorage(ctx context.Context, id string) (SpaceStorage, error)
	SpaceExists(id string) bool
	CreateSpaceStorage(payload spacestorage.SpaceStorageCreatePayload) (SpaceStorage, error)
}

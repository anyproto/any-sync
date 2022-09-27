package storage

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
)

const CName = "commonspace.storage"

type SpaceStorage interface {
	storage.Provider
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

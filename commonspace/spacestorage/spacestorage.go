//go:generate mockgen -destination mock_spacestorage/mock_spacestorage.go github.com/anytypeio/any-sync/commonspace/spacestorage SpaceStorage
package spacestorage

import (
	"bytes"
	"context"
	"errors"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/any-sync/commonspace/object/acl/liststorage"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/util/cidutil"
	"github.com/anytypeio/any-sync/util/crypto"
	"github.com/gogo/protobuf/proto"
	"strings"
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

func ValidateSpaceStorageCreatePayload(payload SpaceStorageCreatePayload) (err error) {
	// TODO: add proper validation
	return nil
}

func ValidateSpaceHeader(spaceId string, header, identity []byte) (err error) {
	split := strings.Split(spaceId, ".")
	if len(split) != 2 {
		return ErrIncorrectSpaceHeader
	}
	if !cidutil.VerifyCid(header, split[0]) {
		err = objecttree.ErrIncorrectCid
		return
	}
	raw := &spacesyncproto.RawSpaceHeader{}
	err = proto.Unmarshal(header, raw)
	if err != nil {
		return
	}
	payload := &spacesyncproto.SpaceHeader{}
	err = proto.Unmarshal(raw.SpaceHeader, payload)
	if err != nil {
		return
	}
	if identity != nil && !bytes.Equal(identity, payload.Identity) {
		err = ErrIncorrectSpaceHeader
		return
	}
	key, err := crypto.NewSigningEd25519PubKeyFromBytes(payload.Identity)
	if err != nil {
		return
	}
	res, err := key.Verify(raw.SpaceHeader, raw.Signature)
	if err != nil || !res {
		err = ErrIncorrectSpaceHeader
		return
	}
	return
}

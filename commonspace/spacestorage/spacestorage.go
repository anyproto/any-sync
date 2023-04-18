//go:generate mockgen -destination mock_spacestorage/mock_spacestorage.go github.com/anytypeio/any-sync/commonspace/spacestorage SpaceStorage
package spacestorage

import (
	"context"
	"errors"
	"fmt"
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
	"strconv"
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
	err = validateCreateSpaceHeaderPayload(payload.SpaceHeaderWithId)
	if err != nil {
		return
	}
	aclSpaceId, err := validateCreateSpaceAclPayload(payload.AclWithId)
	if err != nil {
		return
	}
	aclHeadId, settingsSpaceId, err := validateCreateSpaceSettingsPayload(payload.SpaceSettingsWithId)
	if err != nil {
		return
	}
	if aclSpaceId != payload.SpaceHeaderWithId.Id || aclSpaceId != settingsSpaceId {
		err = ErrIncorrectSpaceHeader
		return
	}
	if aclHeadId != payload.AclWithId.Id {
		err = ErrIncorrectSpaceHeader
		return
	}
	return
}

func validateCreateSpaceHeaderPayload(rawHeaderWithId *spacesyncproto.RawSpaceHeaderWithId) (err error) {
	var rawSpaceHeader spacesyncproto.RawSpaceHeader
	err = proto.Unmarshal(rawHeaderWithId.RawHeader, &rawSpaceHeader)
	if err != nil {
		return
	}
	var header spacesyncproto.SpaceHeader
	err = proto.Unmarshal(rawSpaceHeader.SpaceHeader, &header)
	if err != nil {
		return
	}
	split := strings.Split(rawHeaderWithId.Id, ".")
	if len(split) != 2 {
		return ErrIncorrectSpaceHeader
	}
	if !cidutil.VerifyCid(rawHeaderWithId.RawHeader, split[0]) {
		err = objecttree.ErrIncorrectCid
		return
	}
	payloadIdentity, err := crypto.UnmarshalEd25519PublicKeyProto(header.Identity)
	if err != nil {
		return
	}
	res, err := payloadIdentity.Verify(rawSpaceHeader.SpaceHeader, rawSpaceHeader.Signature)
	if err != nil || !res {
		err = ErrIncorrectSpaceHeader
		return
	}
	id, err := cidutil.NewCidFromBytes(rawHeaderWithId.RawHeader)
	if err != nil {
		return
	}
	requiredSpaceId := fmt.Sprintf("%s.%s", id, strconv.FormatUint(header.ReplicationKey, 36))
	if requiredSpaceId != rawHeaderWithId.Id {
		err = ErrIncorrectSpaceHeader
		return
	}

	return
}

func validateCreateSpaceAclPayload(rawWithId *aclrecordproto.RawAclRecordWithId) (spaceId string, err error) {
	if !cidutil.VerifyCid(rawWithId.Payload, rawWithId.Id) {
		err = objecttree.ErrIncorrectCid
		return
	}
	var rawAcl aclrecordproto.RawAclRecord
	err = proto.Unmarshal(rawWithId.Payload, &rawAcl)
	if err != nil {
		return
	}
	var aclRoot aclrecordproto.AclRoot
	err = proto.Unmarshal(rawAcl.Payload, &aclRoot)
	if err != nil {
		return
	}
	payloadIdentity, err := crypto.UnmarshalEd25519PublicKeyProto(aclRoot.Identity)
	if err != nil {
		return
	}
	res, err := payloadIdentity.Verify(rawAcl.Payload, rawAcl.Signature)
	if err != nil || !res {
		err = ErrIncorrectSpaceHeader
		return
	}
	masterKey, err := crypto.UnmarshalEd25519PublicKeyProto(aclRoot.MasterKey)
	if err != nil {
		return
	}
	rawIdentity, err := payloadIdentity.Raw()
	if err != nil {
		return
	}
	res, err = masterKey.Verify(rawIdentity, aclRoot.IdentitySignature)
	if err != nil || !res {
		err = ErrIncorrectSpaceHeader
		return
	}
	spaceId = aclRoot.SpaceId

	return
}

func validateCreateSpaceSettingsPayload(rawWithId *treechangeproto.RawTreeChangeWithId) (aclHeadId string, spaceId string, err error) {
	var raw treechangeproto.RawTreeChange
	err = proto.Unmarshal(rawWithId.RawChange, &raw)
	if err != nil {
		return
	}
	var rootChange treechangeproto.RootChange
	err = proto.Unmarshal(raw.Payload, &rootChange)
	if err != nil {
		return
	}
	payloadIdentity, err := crypto.UnmarshalEd25519PublicKeyProto(rootChange.Identity)
	if err != nil {
		return
	}
	res, err := payloadIdentity.Verify(raw.Payload, raw.Signature)
	if err != nil || !res {
		err = ErrIncorrectSpaceHeader
		return
	}
	id, err := cidutil.NewCidFromBytes(rawWithId.RawChange)
	if id != rawWithId.Id {
		err = ErrIncorrectSpaceHeader
		return
	}
	spaceId = rootChange.SpaceId
	aclHeadId = rootChange.AclHeadId

	return
}

// ValidateSpaceHeader Used in coordinator
func ValidateSpaceHeader(spaceId string, header []byte, identity crypto.PubKey) (err error) {
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
	payloadIdentity, err := crypto.UnmarshalEd25519PublicKeyProto(payload.Identity)
	if err != nil {
		return
	}
	if identity != nil && !payloadIdentity.Equals(identity) {
		err = ErrIncorrectSpaceHeader
		return
	}
	res, err := identity.Verify(raw.SpaceHeader, raw.Signature)
	if err != nil || !res {
		err = ErrIncorrectSpaceHeader
		return
	}
	return
}

package list

import (
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/util/crypto"
)

type AclStatus int

const (
	StatusNone AclStatus = iota
	StatusJoining
	StatusActive
	StatusRemoved
	StatusDeclined
	StatusRemoving
	StatusCanceled
)

type AclRecord struct {
	Id                string
	PrevId            string
	Timestamp         int64
	AcceptorTimestamp int64
	Data              []byte
	Identity          crypto.PubKey
	Model             interface{}
	Signature         []byte
}

type RequestRecord struct {
	RequestIdentity crypto.PubKey
	RequestMetadata []byte
	KeyRecordId     string
	RecordId        string
	Type            RequestType
}

type AclAccountState struct {
	PubKey          crypto.PubKey
	Permissions     AclPermissions
	RequestMetadata []byte
	KeyRecordId     string
}

type PermissionChange struct {
	RecordId   string
	Permission AclPermissions
}

type AccountState struct {
	PubKey            crypto.PubKey
	Permissions       AclPermissions
	Status            AclStatus
	RequestMetadata   []byte
	KeyRecordId       string
	PermissionChanges []PermissionChange
}

type RequestType int

const (
	RequestTypeRemove RequestType = iota
	RequestTypeJoin
)

type AclPermissions aclrecordproto.AclUserPermissions

const (
	AclPermissionsNone   = AclPermissions(aclrecordproto.AclUserPermissions_None)
	AclPermissionsReader = AclPermissions(aclrecordproto.AclUserPermissions_Reader)
	AclPermissionsGuest  = AclPermissions(aclrecordproto.AclUserPermissions_Guest) // like reader, but can't remove itself and can't request permissions change
	AclPermissionsWriter = AclPermissions(aclrecordproto.AclUserPermissions_Writer)
	AclPermissionsAdmin  = AclPermissions(aclrecordproto.AclUserPermissions_Admin)
	AclPermissionsOwner  = AclPermissions(aclrecordproto.AclUserPermissions_Owner)
)

func (p AclPermissions) NoPermissions() bool {
	return aclrecordproto.AclUserPermissions(p) == aclrecordproto.AclUserPermissions_None
}

func (p AclPermissions) IsOwner() bool {
	return aclrecordproto.AclUserPermissions(p) == aclrecordproto.AclUserPermissions_Owner
}

func (p AclPermissions) CanWrite() bool {
	switch aclrecordproto.AclUserPermissions(p) {
	case aclrecordproto.AclUserPermissions_Admin:
		return true
	case aclrecordproto.AclUserPermissions_Writer:
		return true
	case aclrecordproto.AclUserPermissions_Owner:
		return true
	default:
		return false
	}
}

func (p AclPermissions) CanManageAccounts() bool {
	switch aclrecordproto.AclUserPermissions(p) {
	case aclrecordproto.AclUserPermissions_Admin:
		return true
	case aclrecordproto.AclUserPermissions_Owner:
		return true
	default:
		return false
	}
}

func (p AclPermissions) CanRequestRemove() bool {
	switch aclrecordproto.AclUserPermissions(p) {
	case aclrecordproto.AclUserPermissions_Guest:
		return false
	default:
		return true
	}
}

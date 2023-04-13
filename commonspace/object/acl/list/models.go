package list

import (
	"github.com/anytypeio/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/any-sync/util/crypto"
)

type AclRecord struct {
	Id        string
	PrevId    string
	ReadKeyId string
	Timestamp int64
	Data      []byte
	Identity  crypto.PubKey
	Model     interface{}
	Signature []byte
}

type AclUserState struct {
	PubKey      crypto.PubKey
	Permissions aclrecordproto.AclUserPermissions
}

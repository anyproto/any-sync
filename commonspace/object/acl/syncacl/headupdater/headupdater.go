package headupdater

import "github.com/anyproto/any-sync/commonspace/object/acl/list"

type AclUpdater interface {
	UpdateAcl(aclList list.AclList)
}

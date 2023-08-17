package headupdater

import "github.com/anyproto/any-sync/commonspace/object/acl/list"

type HeadUpdater interface {
	UpdateHeads(id string, heads []string)
}

type AclUpdater interface {
	UpdateAcl(aclList list.AclList)
}

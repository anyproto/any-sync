package exporter

import (
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/liststorage"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
)

type TreeImportParams struct {
	ListStorage     liststorage.ListStorage
	TreeStorage     treestorage.TreeStorage
	BeforeId        string
	IncludeBeforeId bool
}

func ImportHistoryTree(params TreeImportParams) (tree objecttree.ReadableObjectTree, err error) {
	aclList, err := list.BuildAclList(params.ListStorage)
	if err != nil {
		return
	}
	return objecttree.BuildNonVerifiableHistoryTree(objecttree.HistoryTreeParams{
		TreeStorage:     params.TreeStorage,
		AclList:         aclList,
		BeforeId:        params.BeforeId,
		IncludeBeforeId: params.IncludeBeforeId,
	})
}

package exporter

import (
	"github.com/anytypeio/any-sync/commonspace/object/acl/liststorage"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
)

type TreeImportParams struct {
	ListStorage     liststorage.ListStorage
	TreeStorage     treestorage.TreeStorage
	BeforeId        string
	IncludeBeforeId bool
}

func ImportHistoryTree(params TreeImportParams) (objecttree.ReadableObjectTree, error) {
	return nil, nil
}

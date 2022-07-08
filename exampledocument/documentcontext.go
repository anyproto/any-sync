package exampledocument

import "github.com/anytypeio/go-anytype-infrastructure-experiments/acltree"

type documentContext struct {
	aclTree  *acltree.tree // TODO: remove it, because we don't use it
	fullTree *acltree.tree
	aclState *acltree.aclState
	docState DocumentState
}

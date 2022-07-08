package exampledocument

import "github.com/anytypeio/go-anytype-infrastructure-experiments/acltree"

type documentContext struct {
	aclTree  *acltree.Tree // TODO: remove it, because we don't use it
	fullTree *acltree.Tree
	aclState *acltree.aclState
	docState DocumentState
}

package acltree

type documentContext struct {
	aclTree  *Tree // TODO: remove it, because we don't use it
	fullTree *Tree
	aclState *aclState
	docState DocumentState
}

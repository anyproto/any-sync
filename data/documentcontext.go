package data

type documentContext struct {
	aclTree  *Tree // TODO: remove it, because we don't use it
	fullTree *Tree
	aclState *ACLState
	docState DocumentState
}

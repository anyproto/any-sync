package data

type documentContext struct {
	aclTree  *Tree
	fullTree *Tree
	aclState *ACLState
	docState DocumentState
}

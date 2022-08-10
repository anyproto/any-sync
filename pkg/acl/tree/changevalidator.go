package tree

type DocTreeValidator interface {
	ValidateTree(tree *Tree, aclTree ACLTree) error
}

type docTreeValidator struct{}

func newTreeValidator() DocTreeValidator {
	return &docTreeValidator{}
}
func (v *docTreeValidator) ValidateTree(tree *Tree, aclTree ACLTree) error {
	// TODO: add validation logic where we check that the change refers to correct acl heads
	//  that means that more recent changes should refer to more recent acl heads
	return nil
}

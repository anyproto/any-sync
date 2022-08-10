package tree

type ChangeValidator interface {
	ValidateChange(change *Change) error
}

type defChangeValidator struct {
	aclTree ACLTree
}

func NewDefChangeValidator(aclTree ACLTree) ChangeValidator {
	return &defChangeValidator{}
}

func (c *defChangeValidator) ValidateChange(change *Change) error {
	// TODO: add validation logic where we check that the change refers to correct acl heads
	//  that means that more recent changes should refer to more recent acl heads
	return nil
}

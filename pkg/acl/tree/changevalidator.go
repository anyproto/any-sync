package tree

import "github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/list"

type DocTreeValidator interface {
	ValidateTree(tree *Tree, aclList list.ACLList) error
}

type docTreeValidator struct{}

func newTreeValidator() DocTreeValidator {
	return &docTreeValidator{}
}
func (v *docTreeValidator) ValidateTree(tree *Tree, list list.ACLList) error {
	// TODO: add validation logic where we check that the change refers to correct acl heads
	//  that means that more recent changes should refer to more recent acl heads
	return nil
}

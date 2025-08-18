package objecttree

import (
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
)

func IsEmptyDerivedTree(tree ObjectTree) bool {
	return tree.IsDerived() && IsEmptyTree(tree)
}

func IsEmptyTree(tree ObjectTree) bool {
	return tree.Len() == 1 && tree.Root().Id == tree.Id()
}

func IsDerivedRoot(root *treechangeproto.RawTreeChangeWithId) (derived bool, err error) {
	rawChange := &treechangeproto.RawTreeChange{}
	err = rawChange.UnmarshalVT(root.RawChange)
	if err != nil {
		return
	}
	return rawChange.Signature == nil, err
}

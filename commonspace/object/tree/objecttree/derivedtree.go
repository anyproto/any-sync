package objecttree

import (
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/gogo/protobuf/proto"
)

func IsEmptyDerivedTree(tree ObjectTree) bool {
	return tree.IsDerived() && tree.Len() == 1 && tree.Root().Id == tree.Header().Id
}

func IsDerivedRoot(root *treechangeproto.RawTreeChangeWithId) (derived bool, err error) {
	rawChange := &treechangeproto.RawTreeChange{}
	err = proto.Unmarshal(root.RawChange, rawChange)
	if err != nil {
		return
	}
	return rawChange.Signature == nil, err
}

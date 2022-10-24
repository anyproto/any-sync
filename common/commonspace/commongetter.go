package commonspace

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/objectgetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncacl"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
)

type commonSpaceGetter struct {
	spaceId    string
	aclList    *syncacl.SyncACL
	treeGetter treegetter.TreeGetter
}

func newCommonSpaceGetter(spaceId string, aclList *syncacl.SyncACL, treeGetter treegetter.TreeGetter) objectgetter.ObjectGetter {
	return &commonSpaceGetter{
		spaceId:    spaceId,
		aclList:    aclList,
		treeGetter: treeGetter,
	}
}

func (c *commonSpaceGetter) GetObject(ctx context.Context, objectId string) (obj objectgetter.Object, err error) {
	if c.aclList.ID() == objectId {
		obj = c.aclList
		return
	}
	t, err := c.treeGetter.GetTree(ctx, c.spaceId, objectId)
	if err != nil {
		return
	}
	obj = t.(objectgetter.Object)
	return
}

package commonspace

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/objectgetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/settings"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncacl"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
)

type commonSpaceGetter struct {
	spaceId    string
	aclList    *syncacl.SyncACL
	treeGetter treegetter.TreeGetter
	settings   settings.SettingsObject
}

func newCommonSpaceGetter(spaceId string, aclList *syncacl.SyncACL, treeGetter treegetter.TreeGetter, settings settings.SettingsObject) objectgetter.ObjectGetter {
	return &commonSpaceGetter{
		spaceId:    spaceId,
		aclList:    aclList,
		treeGetter: treeGetter,
		settings:   settings,
	}
}

func (c *commonSpaceGetter) GetObject(ctx context.Context, objectId string) (obj objectgetter.Object, err error) {
	if c.aclList.ID() == objectId {
		obj = c.aclList
		return
	}
	if c.settings.ID() == objectId {
		obj = c.settings.(objectgetter.Object)
		return
	}
	t, err := c.treeGetter.GetTree(ctx, c.spaceId, objectId)
	if err != nil {
		return
	}
	obj = t.(objectgetter.Object)
	return
}

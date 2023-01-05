package commonspace

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/object/acl/syncacl"
	"github.com/anytypeio/any-sync/commonspace/object/syncobjectgetter"
	"github.com/anytypeio/any-sync/commonspace/object/treegetter"
	"github.com/anytypeio/any-sync/commonspace/settings"
)

type commonSpaceGetter struct {
	spaceId    string
	aclList    *syncacl.SyncAcl
	treeGetter treegetter.TreeGetter
	settings   settings.SettingsObject
}

func newCommonSpaceGetter(spaceId string, aclList *syncacl.SyncAcl, treeGetter treegetter.TreeGetter, settings settings.SettingsObject) syncobjectgetter.SyncObjectGetter {
	return &commonSpaceGetter{
		spaceId:    spaceId,
		aclList:    aclList,
		treeGetter: treeGetter,
		settings:   settings,
	}
}

func (c *commonSpaceGetter) GetObject(ctx context.Context, objectId string) (obj syncobjectgetter.SyncObject, err error) {
	if c.aclList.Id() == objectId {
		obj = c.aclList
		return
	}
	if c.settings.Id() == objectId {
		obj = c.settings.(syncobjectgetter.SyncObject)
		return
	}
	t, err := c.treeGetter.GetTree(ctx, c.spaceId, objectId)
	if err != nil {
		return
	}
	obj = t.(syncobjectgetter.SyncObject)
	return
}

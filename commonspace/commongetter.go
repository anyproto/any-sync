package commonspace

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/object/syncobjectgetter"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/treegetter"
	"golang.org/x/exp/slices"
)

type commonGetter struct {
	treegetter.TreeGetter
	spaceId         string
	reservedObjects []syncobjectgetter.SyncObject
}

func newCommonGetter(spaceId string, getter treegetter.TreeGetter) *commonGetter {
	return &commonGetter{
		TreeGetter: getter,
		spaceId:    spaceId,
	}
}

func (c *commonGetter) AddObject(object syncobjectgetter.SyncObject) {
	c.reservedObjects = append(c.reservedObjects, object)
}

func (c *commonGetter) GetTree(ctx context.Context, spaceId, treeId string) (objecttree.ObjectTree, error) {
	if obj := c.getReservedObject(treeId); obj != nil {
		return obj.(objecttree.ObjectTree), nil
	}
	return c.TreeGetter.GetTree(ctx, spaceId, treeId)
}

func (c *commonGetter) getReservedObject(id string) syncobjectgetter.SyncObject {
	pos := slices.IndexFunc(c.reservedObjects, func(object syncobjectgetter.SyncObject) bool {
		return object.Id() == id
	})
	if pos == -1 {
		return nil
	}
	return c.reservedObjects[pos]
}

func (c *commonGetter) GetObject(ctx context.Context, objectId string) (obj syncobjectgetter.SyncObject, err error) {
	if obj := c.getReservedObject(objectId); obj != nil {
		return obj, nil
	}
	t, err := c.TreeGetter.GetTree(ctx, c.spaceId, objectId)
	if err != nil {
		return
	}
	obj = t.(syncobjectgetter.SyncObject)
	return
}

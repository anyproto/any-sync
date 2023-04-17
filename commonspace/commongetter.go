package commonspace

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/object/syncobjectgetter"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/treemanager"
	"sync/atomic"
)

type commonGetter struct {
	treemanager.TreeManager
	spaceId         string
	reservedObjects []syncobjectgetter.SyncObject
	spaceIsClosed   *atomic.Bool
}

func newCommonGetter(spaceId string, getter treemanager.TreeManager, spaceIsClosed *atomic.Bool) *commonGetter {
	return &commonGetter{
		TreeManager:   getter,
		spaceId:       spaceId,
		spaceIsClosed: spaceIsClosed,
	}
}

func (c *commonGetter) AddObject(object syncobjectgetter.SyncObject) {
	c.reservedObjects = append(c.reservedObjects, object)
}

func (c *commonGetter) GetTree(ctx context.Context, spaceId, treeId string) (objecttree.ObjectTree, error) {
	if c.spaceIsClosed.Load() {
		return nil, ErrSpaceClosed
	}
	if obj := c.getReservedObject(treeId); obj != nil {
		return obj.(objecttree.ObjectTree), nil
	}
	return c.TreeManager.GetTree(ctx, spaceId, treeId)
}

func (c *commonGetter) getReservedObject(id string) syncobjectgetter.SyncObject {
	for _, obj := range c.reservedObjects {
		if obj != nil && obj.Id() == id {
			return obj
		}
	}
	return nil
}

func (c *commonGetter) GetObject(ctx context.Context, objectId string) (obj syncobjectgetter.SyncObject, err error) {
	if c.spaceIsClosed.Load() {
		return nil, ErrSpaceClosed
	}
	if obj := c.getReservedObject(objectId); obj != nil {
		return obj, nil
	}
	t, err := c.TreeManager.GetTree(ctx, c.spaceId, objectId)
	if err != nil {
		return
	}
	obj = t.(syncobjectgetter.SyncObject)
	return
}

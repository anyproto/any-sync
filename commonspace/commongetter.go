package commonspace

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/syncobjectgetter"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"sync/atomic"
)

type ObjectManager interface {
	treemanager.TreeManager
	AddObject(object syncobjectgetter.SyncObject)
	GetObject(ctx context.Context, objectId string) (obj syncobjectgetter.SyncObject, err error)
}

type objectManager struct {
	treemanager.TreeManager
	spaceId         string
	reservedObjects []syncobjectgetter.SyncObject
	spaceIsClosed   *atomic.Bool
}

func NewObjectManager(manager treemanager.TreeManager) ObjectManager {
	return &objectManager{
		TreeManager: manager,
	}
}

func (c *objectManager) Init(a *app.App) (err error) {
	state := a.MustComponent(spacestate.CName).(*spacestate.SpaceState)
	c.spaceId = state.SpaceId
	c.spaceIsClosed = state.SpaceIsClosed
	return nil
}

func (c *objectManager) Run(ctx context.Context) (err error) {
	return nil
}

func (c *objectManager) Close(ctx context.Context) (err error) {
	return nil
}

func (c *objectManager) AddObject(object syncobjectgetter.SyncObject) {
	c.reservedObjects = append(c.reservedObjects, object)
}

func (c *objectManager) GetTree(ctx context.Context, spaceId, treeId string) (objecttree.ObjectTree, error) {
	if c.spaceIsClosed.Load() {
		return nil, ErrSpaceClosed
	}
	if obj := c.getReservedObject(treeId); obj != nil {
		return obj.(objecttree.ObjectTree), nil
	}
	return c.TreeManager.GetTree(ctx, spaceId, treeId)
}

func (c *objectManager) getReservedObject(id string) syncobjectgetter.SyncObject {
	for _, obj := range c.reservedObjects {
		if obj != nil && obj.Id() == id {
			return obj
		}
	}
	return nil
}

func (c *objectManager) GetObject(ctx context.Context, objectId string) (obj syncobjectgetter.SyncObject, err error) {
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

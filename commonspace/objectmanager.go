package commonspace

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/object/syncobjectgetter"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/settings"
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

func (o *objectManager) Init(a *app.App) (err error) {
	state := a.MustComponent(spacestate.CName).(*spacestate.SpaceState)
	o.spaceId = state.SpaceId
	o.spaceIsClosed = state.SpaceIsClosed
	settingsObject := a.MustComponent(settings.CName).(settings.Settings).SettingsObject()
	acl := a.MustComponent(syncacl.CName).(*syncacl.SyncAcl)
	o.AddObject(settingsObject)
	o.AddObject(acl)
	return nil
}

func (o *objectManager) Run(ctx context.Context) (err error) {
	return nil
}

func (o *objectManager) Close(ctx context.Context) (err error) {
	return nil
}

func (o *objectManager) AddObject(object syncobjectgetter.SyncObject) {
	o.reservedObjects = append(o.reservedObjects, object)
}

func (o *objectManager) GetTree(ctx context.Context, spaceId, treeId string) (objecttree.ObjectTree, error) {
	if o.spaceIsClosed.Load() {
		return nil, ErrSpaceClosed
	}
	if obj := o.getReservedObject(treeId); obj != nil {
		return obj.(objecttree.ObjectTree), nil
	}
	return o.TreeManager.GetTree(ctx, spaceId, treeId)
}

func (o *objectManager) getReservedObject(id string) syncobjectgetter.SyncObject {
	for _, obj := range o.reservedObjects {
		if obj != nil && obj.Id() == id {
			return obj
		}
	}
	return nil
}

func (o *objectManager) GetObject(ctx context.Context, objectId string) (obj syncobjectgetter.SyncObject, err error) {
	if o.spaceIsClosed.Load() {
		return nil, ErrSpaceClosed
	}
	if obj := o.getReservedObject(objectId); obj != nil {
		return obj, nil
	}
	t, err := o.TreeManager.GetTree(ctx, o.spaceId, objectId)
	if err != nil {
		return
	}
	obj = t.(syncobjectgetter.SyncObject)
	return
}

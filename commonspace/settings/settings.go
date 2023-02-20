//go:generate mockgen -destination mock_settings/mock_settings.go github.com/anytypeio/any-sync/commonspace/settings DeletedIdsProvider,Deleter
package settings

import (
	"context"
	"errors"
	"github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/synctree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anytypeio/any-sync/commonspace/object/treegetter"
	"github.com/anytypeio/any-sync/commonspace/settings/settingsstate"
	spacestorage "github.com/anytypeio/any-sync/commonspace/spacestorage"
	"go.uber.org/zap"
	"time"
)

var log = logger.NewNamed("common.commonspace.settings")

const spaceDeletionInterval = time.Hour * 24 * 7

type SettingsObject interface {
	synctree.SyncTree
	Init(ctx context.Context) (err error)
	DeleteObject(id string) (err error)
	DeleteSpace(t time.Time) (err error)
	RestoreSpace() (err error)
}

var (
	ErrDeleteSelf       = errors.New("cannot delete self")
	ErrAlreadyDeleted   = errors.New("the object is already deleted")
	ErrObjDoesNotExist  = errors.New("the object does not exist")
	ErrMarkedDeleted    = errors.New("this space is already marked deleted")
	ErrMarkedNotDeleted = errors.New("this space is not marked not deleted")
)

type BuildTreeFunc func(ctx context.Context, id string, listener updatelistener.UpdateListener) (t synctree.SyncTree, err error)

type Deps struct {
	BuildFunc     BuildTreeFunc
	Account       accountservice.Service
	TreeGetter    treegetter.TreeGetter
	Store         spacestorage.SpaceStorage
	DeletionState settingsstate.ObjectDeletionState
	Provider      SpaceIdsProvider
	OnSpaceDelete func()
	// testing dependencies
	builder       settingsstate.StateBuilder
	del           Deleter
	delManager    DeletionManager
	changeFactory settingsstate.ChangeFactory
}

type settingsObject struct {
	synctree.SyncTree
	account    accountservice.Service
	spaceId    string
	treeGetter treegetter.TreeGetter
	store      spacestorage.SpaceStorage
	builder    settingsstate.StateBuilder
	buildFunc  BuildTreeFunc
	loop       *deleteLoop

	state           *settingsstate.State
	deletionState   settingsstate.ObjectDeletionState
	deletionManager DeletionManager
	changeFactory   settingsstate.ChangeFactory
}

func NewSettingsObject(deps Deps, spaceId string) (obj SettingsObject) {
	var (
		deleter         Deleter
		deletionManager DeletionManager
		builder         settingsstate.StateBuilder
		changeFactory   settingsstate.ChangeFactory
	)
	if deps.del == nil {
		deleter = newDeleter(deps.Store, deps.DeletionState, deps.TreeGetter)
	} else {
		deleter = deps.del
	}
	if deps.delManager == nil {
		deletionManager = newDeletionManager(spaceId, spaceDeletionInterval, deps.DeletionState, deps.Provider, deps.OnSpaceDelete)
	} else {
		deletionManager = deps.delManager
	}
	if deps.builder == nil {
		builder = settingsstate.NewStateBuilder()
	} else {
		builder = deps.builder
	}
	if deps.changeFactory == nil {
		changeFactory = settingsstate.NewChangeFactory()
	} else {
		changeFactory = deps.changeFactory
	}

	loop := newDeleteLoop(func() {
		deleter.Delete()
	})
	deps.DeletionState.AddObserver(func(ids []string) {
		loop.notify()
	})

	s := &settingsObject{
		loop:            loop,
		spaceId:         spaceId,
		account:         deps.Account,
		deletionState:   deps.DeletionState,
		treeGetter:      deps.TreeGetter,
		store:           deps.Store,
		buildFunc:       deps.BuildFunc,
		builder:         builder,
		deletionManager: deletionManager,
		changeFactory:   changeFactory,
	}
	obj = s
	return
}

func (s *settingsObject) updateIds(tr objecttree.ObjectTree, isUpdate bool) {
	var err error
	s.state, err = s.builder.Build(tr, s.state, isUpdate)
	if err != nil {
		log.Error("failed to build state", zap.Error(err))
		return
	}
	if err = s.deletionManager.UpdateState(s.state); err != nil {
		log.Error("failed to update state", zap.Error(err))
	}
}

// Update is called as part of UpdateListener interface
func (s *settingsObject) Update(tr objecttree.ObjectTree) {
	s.updateIds(tr, true)
}

// Rebuild is called as part of UpdateListener interface (including when the object is built for the first time, e.g. on Init call)
func (s *settingsObject) Rebuild(tr objecttree.ObjectTree) {
	// at initial build "s" may not contain the object tree, so it is safer to provide it from the function parameter
	s.updateIds(tr, false)
}

func (s *settingsObject) Init(ctx context.Context) (err error) {
	settingsId := s.store.SpaceSettingsId()
	log.Debug("space settings id", zap.String("id", settingsId))
	s.SyncTree, err = s.buildFunc(ctx, settingsId, s)
	if err != nil {
		return
	}

	s.loop.Run()
	return
}

func (s *settingsObject) Close() error {
	s.loop.Close()
	return s.SyncTree.Close()
}

func (s *settingsObject) DeleteSpace(t time.Time) (err error) {
	s.Lock()
	defer s.Unlock()
	if !s.state.SpaceDeletionDate.IsZero() {
		return ErrMarkedDeleted
	}

	// TODO: add snapshot logic
	res, err := s.changeFactory.CreateSpaceDeleteChange(t, s.state, false)
	if err != nil {
		return
	}

	return s.addContent(res)
}

func (s *settingsObject) RestoreSpace() (err error) {
	s.Lock()
	defer s.Unlock()
	if s.state.SpaceDeletionDate.IsZero() {
		return ErrMarkedNotDeleted
	}

	// TODO: add snapshot logic
	res, err := s.changeFactory.CreateSpaceRestoreChange(s.state, false)
	if err != nil {
		return
	}

	return s.addContent(res)
}

func (s *settingsObject) DeleteObject(id string) (err error) {
	s.Lock()
	defer s.Unlock()
	if s.Id() == id {
		err = ErrDeleteSelf
		return
	}
	if s.deletionState.Exists(id) {
		err = ErrAlreadyDeleted
		return nil
	}
	_, err = s.store.TreeStorage(id)
	if err != nil {
		err = ErrObjDoesNotExist
		return
	}

	// TODO: add snapshot logic
	res, err := s.changeFactory.CreateObjectDeleteChange(id, s.state, false)
	if err != nil {
		return
	}

	return s.addContent(res)
}

func (s *settingsObject) addContent(data []byte) (err error) {
	accountData := s.account.Account()
	_, err = s.AddContent(context.Background(), objecttree.SignableChangeContent{
		Data:        data,
		Key:         accountData.SignKey,
		Identity:    accountData.Identity,
		IsSnapshot:  false,
		IsEncrypted: false,
	})
	if err != nil {
		return
	}

	s.Update(s)
	return
}

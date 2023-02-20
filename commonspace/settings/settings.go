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
	"github.com/anytypeio/any-sync/commonspace/settings/deletionstate"
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
}

var (
	ErrDeleteSelf      = errors.New("cannot delete self")
	ErrAlreadyDeleted  = errors.New("the object is already deleted")
	ErrObjDoesNotExist = errors.New("the object does not exist")
)

type BuildTreeFunc func(ctx context.Context, id string, listener updatelistener.UpdateListener) (t synctree.SyncTree, err error)

type Deps struct {
	BuildFunc     BuildTreeFunc
	Account       accountservice.Service
	TreeGetter    treegetter.TreeGetter
	Store         spacestorage.SpaceStorage
	DeletionState deletionstate.DeletionState
	Provider      SpaceIdsProvider
	OnSpaceDelete func()
	// testing dependencies
	builder    StateBuilder
	del        Deleter
	delManager DeletionManager
}

type settingsObject struct {
	synctree.SyncTree
	account    accountservice.Service
	spaceId    string
	treeGetter treegetter.TreeGetter
	store      spacestorage.SpaceStorage
	builder    StateBuilder
	buildFunc  BuildTreeFunc
	loop       *deleteLoop

	deletionState   deletionstate.DeletionState
	deletionManager DeletionManager
}

func NewSettingsObject(deps Deps, spaceId string) (obj SettingsObject) {
	var (
		deleter         Deleter
		deletionManager DeletionManager
		builder         StateBuilder
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
		builder = newStateBuilder()
	} else {
		builder = deps.builder
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
	}
	obj = s
	return
}

func (s *settingsObject) updateIds(tr objecttree.ObjectTree, isUpdate bool) {
	state, err := s.builder.Build(tr, isUpdate)
	if err != nil {
		log.Error("failed to build state", zap.Error(err))
		return
	}
	if err = s.deletionManager.UpdateState(state); err != nil {
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

func (s *settingsObject) DeleteAccount() (err error) {
	return nil
}

func (s *settingsObject) RestoreAccount() (err error) {
	return nil
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
	res, err := s.deletionState.CreateDeleteChange(id, false)
	if err != nil {
		return
	}

	accountData := s.account.Account()
	_, err = s.AddContent(context.Background(), objecttree.SignableChangeContent{
		Data:        res,
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

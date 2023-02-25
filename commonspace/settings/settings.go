//go:generate mockgen -destination mock_settings/mock_settings.go github.com/anytypeio/any-sync/commonspace/settings DeletionManager,Deleter,SpaceIdsProvider
package settings

import (
	"context"
	"errors"
	"github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/synctree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/treegetter"
	"github.com/anytypeio/any-sync/commonspace/settings/settingsstate"
	spacestorage "github.com/anytypeio/any-sync/commonspace/spacestorage"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

var log = logger.NewNamed("common.commonspace.settings")

type SettingsObject interface {
	synctree.SyncTree
	Init(ctx context.Context) (err error)
	DeleteObject(id string) (err error)
	DeleteSpace(ctx context.Context, deleterId string, raw *treechangeproto.RawTreeChangeWithId) (err error)
	SpaceDeleteRawChange(deleterId string) (raw *treechangeproto.RawTreeChangeWithId, err error)
}

var (
	ErrDeleteSelf            = errors.New("cannot delete self")
	ErrAlreadyDeleted        = errors.New("the object is already deleted")
	ErrObjDoesNotExist       = errors.New("the object does not exist")
	ErrCantDeleteSpace       = errors.New("not able to delete space")
	ErrIncorrectDeleteChange = errors.New("incorrect delete change")
)

type BuildTreeFunc func(ctx context.Context, id string, listener updatelistener.UpdateListener) (t synctree.SyncTree, err error)

type Deps struct {
	BuildFunc     BuildTreeFunc
	Account       accountservice.Service
	TreeGetter    treegetter.TreeGetter
	Store         spacestorage.SpaceStorage
	Configuration nodeconf.Configuration
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
		deletionManager = newDeletionManager(
			spaceId,
			deps.Store.SpaceSettingsId(),
			deps.Configuration.IsResponsible(spaceId),
			deps.TreeGetter,
			deps.DeletionState,
			deps.Provider,
			deps.OnSpaceDelete)
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

func (s *settingsObject) updateIds(tr objecttree.ObjectTree) {
	var err error
	s.state, err = s.builder.Build(tr, s.state)
	if err != nil {
		log.Error("failed to build state", zap.Error(err))
		return
	}
	log.Debug("updating object state", zap.String("deleterId", s.state.DeleterId))
	if err = s.deletionManager.UpdateState(context.Background(), s.state); err != nil {
		log.Error("failed to update state", zap.Error(err))
	}
}

// Update is called as part of UpdateListener interface
func (s *settingsObject) Update(tr objecttree.ObjectTree) {
	s.updateIds(tr)
}

// Rebuild is called as part of UpdateListener interface (including when the object is built for the first time, e.g. on Init call)
func (s *settingsObject) Rebuild(tr objecttree.ObjectTree) {
	// at initial build "s" may not contain the object tree, so it is safer to provide it from the function parameter
	s.state = nil
	s.updateIds(tr)
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

func (s *settingsObject) DeleteSpace(ctx context.Context, deleterId string, raw *treechangeproto.RawTreeChangeWithId) (err error) {
	s.Lock()
	defer s.Unlock()
	defer func() {
		log.Debug("finished adding delete change", zap.Error(err))
	}()
	err = s.verifyDeleteSpace(deleterId, raw)
	if err != nil {
		return
	}
	res, err := s.AddRawChanges(ctx, objecttree.RawChangesPayload{
		NewHeads:   []string{raw.Id},
		RawChanges: []*treechangeproto.RawTreeChangeWithId{raw},
	})
	if err != nil {
		return
	}
	if !slices.Contains(res.Heads, raw.Id) {
		err = ErrCantDeleteSpace
		return
	}
	return
}

func (s *settingsObject) SpaceDeleteRawChange(deleterId string) (raw *treechangeproto.RawTreeChangeWithId, err error) {
	data, err := s.changeFactory.CreateSpaceDeleteChange(deleterId, s.state, false)
	if err != nil {
		return
	}
	accountData := s.account.Account()
	return s.PrepareChange(objecttree.SignableChangeContent{
		Data:        data,
		Key:         accountData.SignKey,
		Identity:    accountData.Identity,
		IsSnapshot:  false,
		IsEncrypted: false,
	})
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

func (s *settingsObject) verifyDeleteSpace(peerId string, raw *treechangeproto.RawTreeChangeWithId) (err error) {
	data, err := s.UnpackChange(raw)
	if err != nil {
		return
	}
	content := &spacesyncproto.SettingsData{}
	err = proto.Unmarshal(data, content)
	if err != nil {
		return
	}
	if len(content.GetContent()) != 1 ||
		content.GetContent()[0].GetSpaceDelete() == nil ||
		content.GetContent()[0].GetSpaceDelete().GetDeleterPeerId() != peerId {
		return ErrIncorrectDeleteChange
	}
	return
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

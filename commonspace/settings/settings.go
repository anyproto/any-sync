//go:generate mockgen -destination mock_settings/mock_settings.go github.com/anytypeio/any-sync/commonspace/settings DeletionManager,Deleter,SpaceIdsProvider
package settings

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/any-sync/util/crypto"

	"github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/synctree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/treemanager"
	"github.com/anytypeio/any-sync/commonspace/settings/settingsstate"
	"github.com/anytypeio/any-sync/commonspace/spacestorage"
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
	DeleteSpace(ctx context.Context, raw *treechangeproto.RawTreeChangeWithId) (err error)
	SpaceDeleteRawChange() (raw *treechangeproto.RawTreeChangeWithId, err error)
}

var (
	ErrDeleteSelf      = errors.New("cannot delete self")
	ErrAlreadyDeleted  = errors.New("the object is already deleted")
	ErrObjDoesNotExist = errors.New("the object does not exist")
	ErrCantDeleteSpace = errors.New("not able to delete space")
)

var (
	doSnapshot       = objecttree.DoSnapshot
	buildHistoryTree = func(objTree objecttree.ObjectTree) (objecttree.ReadableObjectTree, error) {
		return objecttree.BuildHistoryTree(objecttree.HistoryTreeParams{
			TreeStorage:   objTree.Storage(),
			AclList:       objTree.AclList(),
			BuildFullTree: true,
		})
	}
)

type BuildTreeFunc func(ctx context.Context, id string, listener updatelistener.UpdateListener) (t synctree.SyncTree, err error)

type Deps struct {
	BuildFunc     BuildTreeFunc
	Account       accountservice.Service
	TreeManager   treemanager.TreeManager
	Store         spacestorage.SpaceStorage
	Configuration nodeconf.NodeConf
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
	account     accountservice.Service
	spaceId     string
	treeManager treemanager.TreeManager
	store       spacestorage.SpaceStorage
	builder     settingsstate.StateBuilder
	buildFunc   BuildTreeFunc
	loop        *deleteLoop

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
		deleter = newDeleter(deps.Store, deps.DeletionState, deps.TreeManager)
	} else {
		deleter = deps.del
	}
	if deps.delManager == nil {
		deletionManager = newDeletionManager(
			spaceId,
			deps.Store.SpaceSettingsId(),
			deps.Configuration.IsResponsible(spaceId),
			deps.TreeManager,
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
		treeManager:     deps.TreeManager,
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
	log.Debug("updating object state", zap.String("deleted by", s.state.DeleterId))
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
	// TODO: remove this check when everybody updates
	if err = s.checkHistoryState(ctx); err != nil {
		return
	}
	s.loop.Run()
	return
}

func (s *settingsObject) checkHistoryState(ctx context.Context) (err error) {
	historyTree, err := buildHistoryTree(s.SyncTree)
	if err != nil {
		return
	}
	fullState, err := s.builder.Build(historyTree, nil)
	if err != nil {
		return
	}
	if len(fullState.DeletedIds) != len(s.state.DeletedIds) {
		log.WarnCtx(ctx, "state does not have all deleted ids",
			zap.Int("fullstate ids", len(fullState.DeletedIds)),
			zap.Int("state ids", len(fullState.DeletedIds)))
		s.state = fullState
		err = s.deletionManager.UpdateState(context.Background(), s.state)
		if err != nil {
			return
		}
	}
	return
}

func (s *settingsObject) Close() error {
	s.loop.Close()
	return s.SyncTree.Close()
}

func (s *settingsObject) DeleteSpace(ctx context.Context, raw *treechangeproto.RawTreeChangeWithId) (err error) {
	s.Lock()
	defer s.Unlock()
	defer func() {
		log.Debug("finished adding delete change", zap.Error(err))
	}()
	err = s.verifyDeleteSpace(raw)
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

func (s *settingsObject) SpaceDeleteRawChange() (raw *treechangeproto.RawTreeChangeWithId, err error) {
	accountData := s.account.Account()
	data, err := s.changeFactory.CreateSpaceDeleteChange(accountData.PeerId, s.state, false)
	if err != nil {
		return
	}
	return s.PrepareChange(objecttree.SignableChangeContent{
		Data:        data,
		Key:         accountData.SignKey,
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
	if s.state.Exists(id) {
		err = ErrAlreadyDeleted
		return nil
	}
	_, err = s.store.TreeStorage(id)
	if err != nil {
		err = ErrObjDoesNotExist
		return
	}
	isSnapshot := doSnapshot(s.Len())
	res, err := s.changeFactory.CreateObjectDeleteChange(id, s.state, isSnapshot)
	if err != nil {
		return
	}

	return s.addContent(res, isSnapshot)
}

func (s *settingsObject) verifyDeleteSpace(raw *treechangeproto.RawTreeChangeWithId) (err error) {
	data, err := s.UnpackChange(raw)
	if err != nil {
		return
	}
	return verifyDeleteContent(data, "")
}

func (s *settingsObject) addContent(data []byte, isSnapshot bool) (err error) {
	accountData := s.account.Account()
	res, err := s.AddContent(context.Background(), objecttree.SignableChangeContent{
		Data:        data,
		Key:         accountData.SignKey,
		IsSnapshot:  isSnapshot,
		IsEncrypted: false,
	})
	if err != nil {
		return
	}
	if res.Mode == objecttree.Rebuild {
		s.Rebuild(s)
	} else {
		s.Update(s)
	}
	return
}

func VerifyDeleteChange(raw *treechangeproto.RawTreeChangeWithId, identity crypto.PubKey, peerId string) (err error) {
	changeBuilder := objecttree.NewChangeBuilder(crypto.NewKeyStorage(), nil)
	res, err := changeBuilder.Unmarshall(raw, true)
	if err != nil {
		return
	}
	if !res.Identity.Equals(identity) {
		return fmt.Errorf("incorrect identity")
	}
	return verifyDeleteContent(res.Data, peerId)
}

func verifyDeleteContent(data []byte, peerId string) (err error) {
	content := &spacesyncproto.SettingsData{}
	err = proto.Unmarshal(data, content)
	if err != nil {
		return
	}
	if len(content.GetContent()) != 1 ||
		content.GetContent()[0].GetSpaceDelete() == nil ||
		(peerId == "" && content.GetContent()[0].GetSpaceDelete().GetDeleterPeerId() == "") ||
		(peerId != "" && content.GetContent()[0].GetSpaceDelete().GetDeleterPeerId() != peerId) {
		return fmt.Errorf("incorrect delete change payload")
	}
	return
}

package settings

import (
	"context"
	"errors"
	"fmt"

	"github.com/anyproto/any-sync/commonspace/deletionmanager"
	"github.com/anyproto/any-sync/util/crypto"

	"github.com/anyproto/protobuf/proto"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/settings/settingsstate"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/nodeconf"
)

var log = logger.NewNamed("common.commonspace.settings")

type SettingsObject interface {
	synctree.SyncTree
	Init(ctx context.Context) (err error)
	DeleteObject(ctx context.Context, id string) (err error)
}

var (
	ErrDeleteSelf              = errors.New("cannot delete self")
	ErrAlreadyDeleted          = errors.New("the object is already deleted")
	ErrObjDoesNotExist         = errors.New("the object does not exist")
	ErrCantDeleteDerivedObject = errors.New("can't delete derived object")
	ErrCantDeleteSpace         = errors.New("not able to delete space")
)

var (
	DoSnapshot       = objecttree.DoSnapshot
	buildHistoryTree = func(objTree objecttree.ObjectTree) (objecttree.ReadableObjectTree, error) {
		return objecttree.BuildHistoryTree(objecttree.HistoryTreeParams{
			Storage: objTree.Storage(),
			AclList: objTree.AclList(),
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
	DelManager    deletionmanager.DeletionManager
	// testing dependencies
	builder       settingsstate.StateBuilder
	changeFactory settingsstate.ChangeFactory
}

type settingsObject struct {
	synctree.SyncTree
	account   accountservice.Service
	spaceId   string
	store     spacestorage.SpaceStorage
	builder   settingsstate.StateBuilder
	buildFunc BuildTreeFunc

	state           *settingsstate.State
	deletionManager deletionmanager.DeletionManager
	changeFactory   settingsstate.ChangeFactory
}

func NewSettingsObject(deps Deps, spaceId string) (obj SettingsObject) {
	var (
		builder       settingsstate.StateBuilder
		changeFactory settingsstate.ChangeFactory
	)
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

	s := &settingsObject{
		spaceId:         spaceId,
		account:         deps.Account,
		store:           deps.Store,
		buildFunc:       deps.BuildFunc,
		builder:         builder,
		deletionManager: deps.DelManager,
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
	if err = s.deletionManager.UpdateState(context.Background(), s.state); err != nil {
		log.Error("failed to update state", zap.Error(err))
	}
}

// Update is called as part of UpdateListener interface
func (s *settingsObject) Update(tr objecttree.ObjectTree) error {
	s.updateIds(tr)
	return nil
}

// Rebuild is called as part of UpdateListener interface (including when the object is built for the first time, e.g. on Init call)
func (s *settingsObject) Rebuild(tr objecttree.ObjectTree) error {
	// at initial build "s" may not contain the object tree, so it is safer to provide it from the function parameter
	s.state = nil
	s.updateIds(tr)
	return nil
}

func (s *settingsObject) Init(ctx context.Context) (err error) {
	state, err := s.store.StateStorage().GetState(ctx)
	if err != nil {
		return
	}
	log.Debug("space settings id", zap.String("id", state.SettingsId))
	s.SyncTree, err = s.buildFunc(ctx, state.SettingsId, s)
	if err != nil {
		return
	}
	// TODO: remove this check when everybody updates
	if err = s.checkHistoryState(ctx); err != nil {
		return
	}
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
	if s.SyncTree != nil {
		return s.SyncTree.Close()
	}
	return nil
}

func (s *settingsObject) DeleteObject(ctx context.Context, id string) (err error) {
	s.Lock()
	defer s.Unlock()
	if s.Id() == id {
		return ErrDeleteSelf
	}
	if s.state.Exists(id) {
		return ErrAlreadyDeleted
	}
	entry, err := s.store.HeadStorage().GetEntry(ctx, id)
	if err != nil {
		return err
	}
	if entry.IsDerived {
		return ErrCantDeleteDerivedObject
	}
	isSnapshot := DoSnapshot(s.Len())
	res, err := s.changeFactory.CreateObjectDeleteChange(id, s.state, isSnapshot)
	if err != nil {
		return
	}

	return s.addContent(res, isSnapshot)
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

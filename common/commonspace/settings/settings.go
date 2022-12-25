//go:generate mockgen -destination mock_settings/mock_settings.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/settings DeletedIdsProvider,Deleter
package settings

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/settings/deletionstate"
	spacestorage "github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"go.uber.org/zap"
)

var log = logger.NewNamed("commonspace.settings")

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
	Account       account.Service
	TreeGetter    treegetter.TreeGetter
	Store         spacestorage.SpaceStorage
	DeletionState deletionstate.DeletionState
	// testing dependencies
	prov DeletedIdsProvider
	del  Deleter
}

type settingsObject struct {
	synctree.SyncTree
	account    account.Service
	spaceId    string
	treeGetter treegetter.TreeGetter
	store      spacestorage.SpaceStorage
	prov       DeletedIdsProvider
	buildFunc  BuildTreeFunc
	loop       *deleteLoop

	deletionState deletionstate.DeletionState
	lastChangeId  string
}

func NewSettingsObject(deps Deps, spaceId string) (obj SettingsObject) {
	var deleter Deleter
	if deps.del == nil {
		deleter = newDeleter(deps.Store, deps.DeletionState, deps.TreeGetter)
	} else {
		deleter = deps.del
	}

	loop := newDeleteLoop(func() {
		deleter.Delete()
	})
	deps.DeletionState.AddObserver(func(ids []string) {
		loop.notify()
	})

	s := &settingsObject{
		loop:          loop,
		spaceId:       spaceId,
		account:       deps.Account,
		deletionState: deps.DeletionState,
		treeGetter:    deps.TreeGetter,
		store:         deps.Store,
		buildFunc:     deps.BuildFunc,
	}

	// this is needed mainly for testing
	if deps.prov == nil {
		s.prov = &provider{}
	} else {
		s.prov = deps.prov
	}

	obj = s
	return
}

func (s *settingsObject) updateIds(tr tree.ObjectTree, lastChangeId string) {
	s.lastChangeId = lastChangeId
	ids, lastId, err := s.prov.ProvideIds(tr, s.lastChangeId)
	if err != nil {
		log.With(zap.Strings("ids", ids), zap.Error(err)).Error("failed to update state")
		return
	}
	s.lastChangeId = lastId
	if err = s.deletionState.Add(ids); err != nil {
		log.With(zap.Strings("ids", ids), zap.Error(err)).Error("failed to queue ids to delete")
	}
}

// Update is called as part of UpdateListener interface
func (s *settingsObject) Update(tr tree.ObjectTree) {
	s.updateIds(tr, s.lastChangeId)
}

// Rebuild is called as part of UpdateListener interface (including when the object is built for the first time, e.g. on Init call)
func (s *settingsObject) Rebuild(tr tree.ObjectTree) {
	// at initial build "s" may not contain the object tree, so it is safer to provide it from the function parameter
	s.updateIds(tr, "")
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

func (s *settingsObject) DeleteObject(id string) (err error) {
	s.Lock()
	defer s.Unlock()
	if s.ID() == id {
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
	_, err = s.AddContent(context.Background(), tree.SignableChangeContent{
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

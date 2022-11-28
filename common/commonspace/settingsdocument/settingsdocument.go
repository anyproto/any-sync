package settingsdocument

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/settingsdocument/deletionstate"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	spacestorage "github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"go.uber.org/zap"
)

var log = logger.NewNamed("commonspace.settingsdocument")

type SettingsDocument interface {
	synctree.SyncTree
	Init(ctx context.Context) (err error)
	DeleteObject(id string) (err error)
}

type BuildTreeFunc func(ctx context.Context, id string, listener updatelistener.UpdateListener) (t synctree.SyncTree, err error)

type Deps struct {
	BuildFunc     BuildTreeFunc
	Account       account.Service
	TreeGetter    treegetter.TreeGetter
	Store         spacestorage.SpaceStorage
	DeletionState *deletionstate.DeletionState
	// prov exists mainly for the ease of testing
	prov DeletedIdsProvider
}

type settingsDocument struct {
	synctree.SyncTree
	account    account.Service
	spaceId    string
	treeGetter treegetter.TreeGetter
	store      spacestorage.SpaceStorage
	prov       DeletedIdsProvider
	buildFunc  BuildTreeFunc
	loop       *deleteLoop

	deletionState *deletionstate.DeletionState
	lastChangeId  string
}

func NewSettingsDocument(deps Deps, spaceId string) (doc SettingsDocument, err error) {
	deleter := newDeleter(deps.Store, deps.DeletionState, deps.TreeGetter)
	loop := newDeleteLoop(func() {
		deleter.delete()
	})
	deps.DeletionState.AddObserver(func(ids []string) {
		loop.notify()
	})

	s := &settingsDocument{
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
	}

	doc = s
	return
}

func (s *settingsDocument) updateIds(tr tree.ObjectTree, lastChangeId string) {
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

func (s *settingsDocument) Update(tr tree.ObjectTree) {
	s.updateIds(tr, s.lastChangeId)
}

func (s *settingsDocument) Rebuild(tr tree.ObjectTree) {
	// at initial build "s" may not contain the object tree, so it is safer to provide it from the function parameter
	s.updateIds(tr, "")
}

func (s *settingsDocument) Init(ctx context.Context) (err error) {
	s.SyncTree, err = s.buildFunc(ctx, s.store.SpaceSettingsId(), s)
	if err != nil {
		return
	}
	s.loop.Run()
	return
}

func (s *settingsDocument) Close() error {
	s.loop.Close()
	return s.SyncTree.Close()
}

func (s *settingsDocument) DeleteObject(id string) (err error) {
	s.Lock()
	defer s.Unlock()
	if s.deletionState.Exists(id) {
		return nil
	}

	content := &spacesyncproto.SpaceSettingsContent_ObjectDelete{
		ObjectDelete: &spacesyncproto.ObjectDelete{Id: id},
	}
	change := &spacesyncproto.SettingsData{
		Content: []*spacesyncproto.SpaceSettingsContent{
			{content},
		},
		Snapshot: nil,
	}
	res, err := change.Marshal()
	if err != nil {
		return
	}
	_, err = s.AddContent(context.Background(), tree.SignableChangeContent{
		Data:        res,
		Key:         s.account.Account().SignKey,
		Identity:    s.account.Account().Identity,
		IsSnapshot:  false,
		IsEncrypted: false,
	})
	if err == nil {
		s.Update(s)
	}
	return
}

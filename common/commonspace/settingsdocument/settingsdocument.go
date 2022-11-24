package settingsdocument

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/settingsdocument/deletionstate"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	spacestorage "github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"go.uber.org/zap"
)

var log = logger.NewNamed("commonspace.settingsdocument")

type SettingsDocument interface {
	tree.ObjectTree
	Init(ctx context.Context) (err error)
	DeleteObject(id string) (err error)
}

type BuildTreeFunc func(ctx context.Context, id string, listener updatelistener.UpdateListener) (t tree.ObjectTree, err error)

type Deps struct {
	BuildFunc     BuildTreeFunc
	Account       account.Service
	TreeGetter    treegetter.TreeGetter
	Store         spacestorage.SpaceStorage
	DeletionState *deletionstate.DeletionState
	// prov exists mainly for the ease of testing
	prov deletedIdsProvider
}

type settingsDocument struct {
	tree.ObjectTree
	account    account.Service
	spaceId    string
	treeGetter treegetter.TreeGetter
	store      spacestorage.SpaceStorage
	prov       deletedIdsProvider
	buildFunc  BuildTreeFunc
	loop       deleteLoop

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

func (s *settingsDocument) updateIds(lastChangeId string) {
	s.lastChangeId = lastChangeId
	ids, lastId, err := s.prov.ProvideIds(s, s.lastChangeId)
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
	s.updateIds(s.lastChangeId)
}

func (s *settingsDocument) Rebuild(tr tree.ObjectTree) {
	s.updateIds("")
}

func (s *settingsDocument) Init(ctx context.Context) (err error) {
	s.ObjectTree, err = s.buildFunc(ctx, s.store.SpaceSettingsId(), s)
	if err != nil {
		return
	}
	s.loop.Run()
	return
}

func (s *settingsDocument) Close() error {
	s.loop.Close()
	return s.ObjectTree.Close()
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

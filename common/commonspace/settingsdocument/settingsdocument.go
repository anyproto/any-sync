package settingsdocument

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	spacestorage "github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"sync"
)

type DeletionState int

const (
	DeletionStateQueued DeletionState = iota
	DeletionStateDeleted
)

type SettingsDocument interface {
	tree.ObjectTree
	Refresh()
	DeleteObject(id string) (err error)
}

type BuildTreeFunc func(ctx context.Context, id string, listener updatelistener.UpdateListener) (t tree.ObjectTree, err error)
type RemoveObjectsFunc func([]string)

type Deps struct {
	BuildFunc  BuildTreeFunc
	Account    account.Service
	TreeGetter treegetter.TreeGetter
	Store      spacestorage.SpaceStorage
	RemoveFunc RemoveObjectsFunc
	prov       deletedIdsProvider
}

type settingsDocument struct {
	tree.ObjectTree
	account           account.Service
	spaceId           string
	deletionState     map[string]DeletionState
	treeGetter        treegetter.TreeGetter
	store             spacestorage.SpaceStorage
	lastChangeId      string
	prov              deletedIdsProvider
	removeNotifyFunc  RemoveObjectsFunc
	deletionStateLock sync.Mutex
}

func NewSettingsDocument(ctx context.Context, deps Deps, spaceId string) (doc SettingsDocument, err error) {
	s := &settingsDocument{
		account:          deps.Account,
		spaceId:          spaceId,
		deletionState:    map[string]DeletionState{},
		treeGetter:       deps.TreeGetter,
		store:            deps.Store,
		removeNotifyFunc: deps.RemoveFunc,
	}
	s.ObjectTree, err = deps.BuildFunc(ctx, deps.Store.SpaceSettingsId(), s)
	if err != nil {
		return
	}
	// this is needed mainly for testing
	if deps.prov == nil {
		s.prov = &provider{}
	}
	doc = s
	return
}

func (s *settingsDocument) NotifyHeadsUpdate(id string) {
	s.deletionStateLock.Lock()
	if _, exists := s.deletionState[id]; exists {
		s.deletionState[id] = DeletionStateQueued
	}
	s.deletionStateLock.Unlock()
}

func (s *settingsDocument) Update(tr tree.ObjectTree) {
	ids, lastId, err := s.prov.ProvideIds(tr, s.lastChangeId)
	if err != nil {
		return
	}
	s.lastChangeId = lastId
	s.toBeDeleted(ids)
}

func (s *settingsDocument) Rebuild(tr tree.ObjectTree) {
	ids, lastId, err := s.prov.ProvideIds(tr, "")
	if err != nil {
		return
	}
	s.lastChangeId = lastId
	s.toBeDeleted(ids)
}

func (s *settingsDocument) Refresh() {
	s.Lock()
	defer s.Unlock()
	s.Rebuild(s)
}

func (s *settingsDocument) toBeDeleted(ids []string) {
	for _, id := range ids {
		s.deletionStateLock.Lock()
		if state, exists := s.deletionState[id]; exists && state == DeletionStateDeleted {
			s.deletionStateLock.Unlock()
			continue
		}
		// if not already deleted
		// TODO: here we can possibly have problems if the document is synced later, maybe we should block syncing with deleted documents
		if _, err := s.store.TreeStorage(id); err == nil {
			s.deletionState[id] = DeletionStateQueued
			s.deletionStateLock.Unlock()
			// doing this without lock
			err := s.treeGetter.DeleteTree(context.Background(), s.spaceId, id)
			if err != nil {
				// TODO: some errors may tell us that the tree is actually deleted, so we should have more checks here
				// TODO: add logging
				continue
			}
			// TODO: add loop to double check that everything that should be deleted is actually deleted
			s.deletionStateLock.Lock()
		}
		
		s.deletionState[id] = DeletionStateDeleted
		s.deletionStateLock.Unlock()
	}
	// notifying about removal
	s.removeNotifyFunc(ids)
}

func (s *settingsDocument) DeleteObject(id string) (err error) {
	s.Lock()
	defer s.Unlock()
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

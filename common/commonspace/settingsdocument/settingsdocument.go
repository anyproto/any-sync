package settingsdocument

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	spacestorage "github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
)

type SettingsDocument interface {
	tree.ObjectTree
	Init(ctx context.Context) (err error)
	Refresh()
	DeleteObject(id string) (err error)
	NotifyObjectUpdate(id string)
}

type BuildTreeFunc func(ctx context.Context, id string, listener updatelistener.UpdateListener) (t tree.ObjectTree, err error)
type RemoveObjectsFunc func([]string)

type Deps struct {
	BuildFunc  BuildTreeFunc
	Account    account.Service
	TreeGetter treegetter.TreeGetter
	Store      spacestorage.SpaceStorage
	RemoveFunc RemoveObjectsFunc
	// prov exists mainly for the ease of testing
	prov deletedIdsProvider
}

type settingsDocument struct {
	tree.ObjectTree
	account          account.Service
	spaceId          string
	treeGetter       treegetter.TreeGetter
	store            spacestorage.SpaceStorage
	prov             deletedIdsProvider
	removeNotifyFunc RemoveObjectsFunc
	buildFunc        BuildTreeFunc

	queue        *settingsQueue
	documentIds  []string
	lastChangeId string
}

func NewSettingsDocument(deps Deps, spaceId string) (doc SettingsDocument, err error) {
	s := &settingsDocument{
		account:          deps.Account,
		spaceId:          spaceId,
		queue:            newSettingsQueue(),
		treeGetter:       deps.TreeGetter,
		store:            deps.Store,
		removeNotifyFunc: deps.RemoveFunc,
		buildFunc:        deps.BuildFunc,
	}

	// this is needed mainly for testing
	if deps.prov == nil {
		s.prov = &provider{}
	}
	doc = s
	return
}

func (s *settingsDocument) NotifyObjectUpdate(id string) {
	s.queue.queueIfDeleted(id)
}

func (s *settingsDocument) Update(tr tree.ObjectTree) {
	ids, lastId, err := s.prov.ProvideIds(tr, s.lastChangeId)
	if err != nil {
		return
	}
	s.documentIds = append(s.documentIds, ids...)
	s.lastChangeId = lastId
	s.queue.add(ids)
	s.removeNotifyFunc(ids)
	s.deleteQueued()
}

func (s *settingsDocument) Rebuild(tr tree.ObjectTree) {
	ids, lastId, err := s.prov.ProvideIds(tr, "")
	if err != nil {
		return
	}
	s.documentIds = ids
	s.lastChangeId = lastId
	s.queue.add(ids)
	s.removeNotifyFunc(ids)
	s.deleteQueued()
}

func (s *settingsDocument) Init(ctx context.Context) (err error) {
	s.ObjectTree, err = s.buildFunc(ctx, s.store.SpaceSettingsId(), s)
	return
}

func (s *settingsDocument) Refresh() {
	s.Lock()
	defer s.Unlock()
	if s.lastChangeId == "" {
		s.Rebuild(s)
	} else {
		s.deleteQueued()
	}
}

func (s *settingsDocument) deleteQueued() {
	allQueued := s.queue.getQueued()
	for _, id := range allQueued {
		if _, err := s.store.TreeStorage(id); err == nil {
			err := s.treeGetter.DeleteTree(context.Background(), s.spaceId, id)
			if err != nil {
				// TODO: some errors may tell us that the tree is actually deleted, so we should have more checks here
				// TODO: add logging
				continue
			}
		}
		s.queue.delete(id)
	}
}

func (s *settingsDocument) DeleteObject(id string) (err error) {
	s.Lock()
	defer s.Unlock()
	if s.queue.exists(id) {
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

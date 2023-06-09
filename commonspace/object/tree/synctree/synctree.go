//go:generate mockgen -destination mock_synctree/mock_synctree.go github.com/anyproto/any-sync/commonspace/object/tree/synctree SyncTree,ReceiveQueue,HeadNotifiable,SyncClient,RequestFactory,TreeSyncProtocol
package synctree

import (
	"context"
	"errors"
	"time"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/objectsync/synchandler"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/nodeconf"
	"go.uber.org/zap"
)

var (
	ErrSyncTreeClosed  = errors.New("sync tree is closed")
	ErrSyncTreeDeleted = errors.New("sync tree is deleted")
)

type HeadNotifiable interface {
	UpdateHeads(id string, heads []string)
}

type ListenerSetter interface {
	SetListener(listener updatelistener.UpdateListener)
}

type SyncTree interface {
	objecttree.ObjectTree
	synchandler.SyncHandler
	ListenerSetter
	SyncWithPeer(ctx context.Context, peerId string) (err error)
}

// SyncTree sends head updates to sync service and also sends new changes to update listener
type syncTree struct {
	objecttree.ObjectTree
	synchandler.SyncHandler
	syncClient SyncClient
	syncStatus syncstatus.StatusUpdater
	notifiable HeadNotifiable
	listener   updatelistener.UpdateListener
	onClose    func(id string)
	isClosed   bool
	isDeleted  bool
}

var log = logger.NewNamed("common.commonspace.synctree")

type ResponsiblePeersGetter interface {
	GetResponsiblePeers(ctx context.Context) (peers []peer.Peer, err error)
}

type BuildDeps struct {
	SpaceId            string
	SyncClient         SyncClient
	Configuration      nodeconf.NodeConf
	HeadNotifiable     HeadNotifiable
	Listener           updatelistener.UpdateListener
	AclList            list.AclList
	SpaceStorage       spacestorage.SpaceStorage
	TreeStorage        treestorage.TreeStorage
	OnClose            func(id string)
	SyncStatus         syncstatus.StatusUpdater
	PeerGetter         ResponsiblePeersGetter
	BuildObjectTree    objecttree.BuildObjectTreeFunc
	WaitTreeRemoteSync bool
}

func BuildSyncTreeOrGetRemote(ctx context.Context, id string, deps BuildDeps) (t SyncTree, err error) {
	var (
		remoteGetter = treeRemoteGetter{treeId: id, deps: deps}
		isRemote     bool
	)
	deps.TreeStorage, isRemote, err = remoteGetter.getTree(ctx)
	if err != nil {
		return
	}
	return buildSyncTree(ctx, isRemote, deps)
}

func PutSyncTree(ctx context.Context, payload treestorage.TreeStorageCreatePayload, deps BuildDeps) (t SyncTree, err error) {
	deps.TreeStorage, err = deps.SpaceStorage.CreateTreeStorage(payload)
	if err != nil {
		return
	}
	return buildSyncTree(ctx, true, deps)
}

func buildSyncTree(ctx context.Context, sendUpdate bool, deps BuildDeps) (t SyncTree, err error) {
	objTree, err := deps.BuildObjectTree(deps.TreeStorage, deps.AclList)
	if err != nil {
		return
	}
	syncClient := deps.SyncClient
	syncTree := &syncTree{
		ObjectTree: objTree,
		syncClient: syncClient,
		notifiable: deps.HeadNotifiable,
		onClose:    deps.OnClose,
		listener:   deps.Listener,
		syncStatus: deps.SyncStatus,
	}
	syncHandler := newSyncTreeHandler(deps.SpaceId, syncTree, syncClient, deps.SyncStatus)
	syncTree.SyncHandler = syncHandler
	t = syncTree
	syncTree.Lock()
	syncTree.afterBuild()
	syncTree.Unlock()

	if sendUpdate {
		headUpdate := syncTree.syncClient.CreateHeadUpdate(t, nil)
		// send to everybody, because everybody should know that the node or client got new tree
		syncTree.syncClient.Broadcast(headUpdate)
	}
	return
}

func (s *syncTree) SetListener(listener updatelistener.UpdateListener) {
	// this should be called under lock
	s.listener = listener
}

func (s *syncTree) IterateFrom(id string, convert objecttree.ChangeConvertFunc, iterate objecttree.ChangeIterateFunc) (err error) {
	if err = s.checkAlive(); err != nil {
		return
	}
	return s.ObjectTree.IterateFrom(id, convert, iterate)
}

func (s *syncTree) IterateRoot(convert objecttree.ChangeConvertFunc, iterate objecttree.ChangeIterateFunc) (err error) {
	if err = s.checkAlive(); err != nil {
		return
	}
	return s.ObjectTree.IterateRoot(convert, iterate)
}

func (s *syncTree) AddContent(ctx context.Context, content objecttree.SignableChangeContent) (res objecttree.AddResult, err error) {
	if err = s.checkAlive(); err != nil {
		return
	}
	res, err = s.ObjectTree.AddContent(ctx, content)
	if err != nil {
		return
	}
	if s.notifiable != nil {
		s.notifiable.UpdateHeads(s.Id(), res.Heads)
	}
	s.syncStatus.HeadsChange(s.Id(), res.Heads)
	headUpdate := s.syncClient.CreateHeadUpdate(s, res.Added)
	s.syncClient.Broadcast(headUpdate)
	return
}

func (s *syncTree) AddRawChanges(ctx context.Context, changesPayload objecttree.RawChangesPayload) (res objecttree.AddResult, err error) {
	if err = s.checkAlive(); err != nil {
		return
	}
	res, err = s.ObjectTree.AddRawChanges(ctx, changesPayload)
	if err != nil {
		return
	}
	if s.listener != nil {
		switch res.Mode {
		case objecttree.Nothing:
			return
		case objecttree.Append:
			s.listener.Update(s)
		case objecttree.Rebuild:
			s.listener.Rebuild(s)
		}
	}
	if res.Mode != objecttree.Nothing {
		if s.notifiable != nil {
			s.notifiable.UpdateHeads(s.Id(), res.Heads)
		}
		headUpdate := s.syncClient.CreateHeadUpdate(s, res.Added)
		s.syncClient.Broadcast(headUpdate)
	}
	return
}

func (s *syncTree) Delete() (err error) {
	log.Debug("deleting sync tree", zap.String("id", s.Id()))
	defer func() {
		log.Debug("deleted sync tree", zap.Error(err), zap.String("id", s.Id()))
	}()
	s.Lock()
	defer s.Unlock()
	if err = s.checkAlive(); err != nil {
		return
	}
	err = s.ObjectTree.Delete()
	if err != nil {
		return
	}
	s.isDeleted = true
	return
}

func (s *syncTree) TryClose(objectTTL time.Duration) (bool, error) {
	return true, s.Close()
}

func (s *syncTree) Close() (err error) {
	log.Debug("closing sync tree", zap.String("id", s.Id()))
	defer func() {
		log.Debug("closed sync tree", zap.Error(err), zap.String("id", s.Id()))
	}()
	s.Lock()
	defer s.Unlock()
	if s.isClosed {
		return ErrSyncTreeClosed
	}
	s.onClose(s.Id())
	s.isClosed = true
	return
}

func (s *syncTree) checkAlive() (err error) {
	if s.isClosed {
		err = ErrSyncTreeClosed
	}
	if s.isDeleted {
		err = ErrSyncTreeDeleted
	}
	return
}

func (s *syncTree) SyncWithPeer(ctx context.Context, peerId string) (err error) {
	s.Lock()
	defer s.Unlock()
	headUpdate := s.syncClient.CreateHeadUpdate(s, nil)
	return s.syncClient.SendUpdate(peerId, headUpdate.RootChange.Id, headUpdate)
}

func (s *syncTree) afterBuild() {
	if s.listener != nil {
		s.listener.Rebuild(s)
	}
	if s.notifiable != nil {
		s.notifiable.UpdateHeads(s.Id(), s.Heads())
	}
}

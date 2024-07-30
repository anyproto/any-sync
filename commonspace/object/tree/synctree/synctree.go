//go:generate mockgen -destination mock_synctree/mock_synctree.go github.com/anyproto/any-sync/commonspace/object/tree/synctree SyncTree,ReceiveQueue,HeadNotifiable,SyncClient,RequestFactory,TreeSyncProtocol
package synctree

import (
	"context"
	"errors"
	"slices"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/objectsync/synchandler"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/util/slice"
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

type peerSendableObjectTree interface {
	objecttree.ObjectTree
	AddRawChangesFromPeer(ctx context.Context, peerId string, changesPayload objecttree.RawChangesPayload) (res objecttree.AddResult, err error)
}

type SyncTree interface {
	peerSendableObjectTree
	synchandler.SyncHandler
	ListenerSetter
	SyncWithPeer(ctx context.Context, p peer.Peer) (err error)
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
	ValidateObjectTree objecttree.ValidatorFunc
}

func BuildSyncTreeOrGetRemote(ctx context.Context, id string, deps BuildDeps) (t SyncTree, err error) {
	var (
		remoteGetter = treeRemoteGetter{treeId: id, deps: deps}
		peerId       string
	)
	deps.TreeStorage, peerId, err = remoteGetter.getTree(ctx)
	if err != nil {
		return
	}
	return buildSyncTree(ctx, peerId, deps)
}

func PutSyncTree(ctx context.Context, payload treestorage.TreeStorageCreatePayload, deps BuildDeps) (t SyncTree, err error) {
	deps.TreeStorage, err = deps.SpaceStorage.CreateTreeStorage(payload)
	if err != nil {
		return
	}
	return buildSyncTree(ctx, "", deps)
}

func buildSyncTree(ctx context.Context, peerId string, deps BuildDeps) (t SyncTree, err error) {
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

	// don't send updates for empty derived trees, because they won't be accepted
	if peerId != "" && !objecttree.IsEmptyDerivedTree(objTree) {
		headUpdate := syncTree.syncClient.CreateHeadUpdate(t, nil)
		// send to everybody, because everybody should know that the node or client got new tree
		syncTree.syncClient.Broadcast(headUpdate)
		deps.SyncStatus.ObjectReceive(peerId, syncTree.Id(), syncTree.Heads())
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

func (s *syncTree) hasHeads(ot objecttree.ObjectTree, heads []string) bool {
	return slice.UnsortedEquals(ot.Heads(), heads) || ot.HasChanges(heads...)
}

func (s *syncTree) AddRawChangesFromPeer(ctx context.Context, peerId string, changesPayload objecttree.RawChangesPayload) (res objecttree.AddResult, err error) {
	if s.hasHeads(s, changesPayload.NewHeads) {
		s.syncStatus.HeadsApply(peerId, s.Id(), s.Heads(), true)
		return objecttree.AddResult{
			OldHeads: changesPayload.NewHeads,
			Heads:    changesPayload.NewHeads,
			Mode:     objecttree.Nothing,
		}, nil
	}
	prevHeads := s.Heads()
	res, err = s.AddRawChanges(ctx, changesPayload)
	if err != nil {
		return
	}
	curHeads := s.Heads()
	allAdded := true
	for _, head := range changesPayload.NewHeads {
		if !slices.Contains(curHeads, head) {
			allAdded = false
			break
		}
	}
	if !slice.UnsortedEquals(prevHeads, curHeads) {
		s.syncStatus.HeadsApply(peerId, s.Id(), curHeads, allAdded)
	}
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
	if !s.TryLock() {
		return false, nil
	}
	log.Debug("closing sync tree", zap.String("id", s.Id()))
	return true, s.close()
}

func (s *syncTree) Close() (err error) {
	log.Debug("closing sync tree", zap.String("id", s.Id()))
	s.Lock()
	return s.close()
}

func (s *syncTree) close() (err error) {
	defer s.Unlock()
	defer func() {
		log.Debug("closed sync tree", zap.Error(err), zap.String("id", s.Id()))
	}()
	if s.isClosed {
		err = ErrSyncTreeClosed
		return
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

func (s *syncTree) SyncWithPeer(ctx context.Context, p peer.Peer) (err error) {
	s.Lock()
	defer s.Unlock()
	protoVersion, err := peer.CtxProtoVersion(ctx)
	// this works with old protocol
	if err != nil || protoVersion <= secureservice.ProtoVersion {
		headUpdate := s.syncClient.CreateHeadUpdate(s, nil)
		return s.syncClient.SendUpdate(p.Id(), headUpdate.RootChange.Id, headUpdate)
	}
	// for new protocol sending empty request
	request := s.syncClient.CreateEmptyFullSyncRequest(s)
	return s.syncClient.QueueRequest(p.Id(), s.Id(), request)
}

func (s *syncTree) afterBuild() {
	if s.listener != nil {
		s.listener.Rebuild(s)
	}
	if s.notifiable != nil {
		s.notifiable.UpdateHeads(s.Id(), s.Heads())
	}
}

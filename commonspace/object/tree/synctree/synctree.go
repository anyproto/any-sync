//go:generate mockgen -destination mock_synctree/mock_synctree.go github.com/anyproto/any-sync/commonspace/object/tree/synctree SyncTree,HeadNotifiable,SyncClient,RequestFactory
package synctree

import (
	"context"
	"errors"
	"slices"
	"time"

	anystore "github.com/anyproto/any-store"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
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
	syncdeps.ObjectSyncHandler
	AddRawChangesFromPeer(ctx context.Context, peerId string, changesPayload objecttree.RawChangesPayload) (res objecttree.AddResult, err error)
}

type SyncTree interface {
	peerSendableObjectTree
	ListenerSetter
	SyncWithPeer(ctx context.Context, p peer.Peer) (err error)
}

// SyncTree sends head updates to sync service and also sends new changes to update listener
type syncTree struct {
	syncdeps.ObjectSyncHandler
	objecttree.ObjectTree
	syncClient SyncClient
	syncStatus syncstatus.StatusUpdater
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
	TreeStorage        objecttree.Storage
	OnClose            func(id string)
	SyncStatus         syncstatus.StatusUpdater
	PeerGetter         ResponsiblePeersGetter
	BuildObjectTree    objecttree.BuildObjectTreeFunc
	ValidateObjectTree objecttree.ValidatorFunc
}

var newTreeGetter = func(deps BuildDeps, treeId string) treeGetter {
	return treeRemoteGetter{deps: deps, treeId: treeId}
}

func BuildSyncTreeOrGetRemote(ctx context.Context, id string, deps BuildDeps) (t SyncTree, err error) {
	var (
		remoteGetter = newTreeGetter(deps, id)
		peerId       string
	)
	deps.TreeStorage, peerId, err = remoteGetter.getTree(ctx)
	if err != nil {
		return
	}
	return buildSyncTree(ctx, peerId, deps)
}

func PutSyncTree(ctx context.Context, payload treestorage.TreeStorageCreatePayload, deps BuildDeps) (t SyncTree, err error) {
	err = checkTreeDeleted(ctx, payload.RootRawChange.Id, deps.SpaceStorage)
	if err != nil {
		return
	}
	deps.TreeStorage, err = deps.SpaceStorage.CreateTreeStorage(ctx, payload)
	if err != nil {
		return
	}
	return buildSyncTree(ctx, peer.CtxResponsiblePeers, deps)
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
		onClose:    deps.OnClose,
		listener:   deps.Listener,
		syncStatus: deps.SyncStatus,
	}
	syncHandler := NewSyncHandler(syncTree, syncClient, deps.SpaceId)
	syncTree.ObjectSyncHandler = syncHandler
	t = syncTree
	syncTree.Lock()
	syncTree.afterBuild()
	syncTree.Unlock()

	if peerId != "" && !objecttree.IsEmptyDerivedTree(objTree) {
		headUpdate, err := syncTree.syncClient.CreateHeadUpdate(objTree, "", nil)
		if err != nil {
			return nil, err
		}
		log.Debug("broadcast after build", zap.String("treeId", objTree.Id()), zap.Strings("heads", objTree.Heads()))
		// send to everybody, because everybody should know that the node or client got new tree
		broadcastErr := syncTree.syncClient.Broadcast(ctx, headUpdate)
		if broadcastErr != nil {
			log.Warn("failed to broadcast head update", zap.Error(broadcastErr))
		}
		if peerId != peer.CtxResponsiblePeers {
			deps.SyncStatus.ObjectReceive(peerId, syncTree.Id(), syncTree.Heads())
		}
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
	return s.AddContentWithValidator(ctx, content, nil)
}

func (s *syncTree) AddContentWithValidator(ctx context.Context, content objecttree.SignableChangeContent, validate func(change objecttree.StorageChange) error) (res objecttree.AddResult, err error) {
	if err = s.checkAlive(); err != nil {
		return
	}
	res, err = s.ObjectTree.AddContentWithValidator(ctx, content, validate)
	if err != nil {
		return
	}
	s.syncStatus.HeadsChange(s.Id(), res.Heads)
	headUpdate, err := s.syncClient.CreateHeadUpdate(s, "", res.RawChanges())
	if err != nil {
		return
	}
	broadcastErr := s.syncClient.Broadcast(ctx, headUpdate)
	if broadcastErr != nil {
		log.Warn("failed to broadcast head update", zap.Error(broadcastErr))
	}
	return
}

func (s *syncTree) AddRawChangesFromPeer(ctx context.Context, peerId string, changesPayload objecttree.RawChangesPayload) (res objecttree.AddResult, err error) {
	if s.hasHeads(s, changesPayload.NewHeads) {
		s.syncStatus.HeadsApply(peerId, s.Id(), s.Heads(), true)
		log.Debug("heads already applied", zap.String("treeId", s.Id()), zap.Strings("heads", changesPayload.NewHeads))
		return objecttree.AddResult{
			OldHeads: changesPayload.NewHeads,
			Heads:    changesPayload.NewHeads,
			Mode:     objecttree.Nothing,
		}, nil
	}
	prevHeads := s.Heads()
	res, err = s.AddRawChanges(ctx, changesPayload)
	if err != nil || res.Mode == objecttree.Nothing {
		return
	}
	headUpdate, err := s.syncClient.CreateHeadUpdate(s, peerId, res.RawChanges())
	if err != nil {
		return res, err
	}
	broadcastErr := s.syncClient.Broadcast(ctx, headUpdate)
	if broadcastErr != nil {
		log.Warn("failed to broadcast head update", zap.Error(broadcastErr))
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

func (s *syncTree) hasHeads(ot objecttree.ObjectTree, heads []string) bool {
	return slice.UnsortedEquals(ot.Heads(), heads) || ot.HasChanges(heads...)
}

func (s *syncTree) AddRawChanges(ctx context.Context, changesPayload objecttree.RawChangesPayload) (res objecttree.AddResult, err error) {
	if err = s.checkAlive(); err != nil {
		return
	}
	return s.ObjectTree.AddRawChangesWithUpdater(ctx, changesPayload, func(tree objecttree.ObjectTree, md objecttree.Mode) error {
		if s.listener != nil {
			switch md {
			case objecttree.Nothing:
				return nil
			case objecttree.Append:
				return s.listener.Update(s)
			case objecttree.Rebuild:
				return s.listener.Rebuild(s)
			}
		}
		return nil
	})
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
	if objecttree.IsEmptyDerivedTree(s.ObjectTree) {
		return
	}
	protoVersion, err := peer.CtxProtoVersion(p.Context())
	if err != nil {
		return
	}
	if protoVersion <= secureservice.CompatibleVersion {
		return nil
	}
	req := s.syncClient.CreateFullSyncRequest(p.Id(), s)
	return s.syncClient.QueueRequest(ctx, req)
}

func (s *syncTree) afterBuild() {
	if s.listener != nil {
		s.listener.Rebuild(s)
	}
}

func checkTreeDeleted(ctx context.Context, treeId string, spaceStorage spacestorage.SpaceStorage) error {
	status, err := spaceStorage.HeadStorage().GetEntry(ctx, treeId)
	if err != nil {
		if errors.Is(err, anystore.ErrDocNotFound) {
			return nil
		}
		return err
	}
	if status.DeletedStatus != headstorage.DeletedStatusNotDeleted {
		return spacestorage.ErrTreeStorageAlreadyDeleted
	}
	return nil
}

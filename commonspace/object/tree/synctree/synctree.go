package synctree

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/commonspace/object/acl/list"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
	"github.com/anytypeio/any-sync/commonspace/objectsync"
	"github.com/anytypeio/any-sync/commonspace/objectsync/synchandler"
	spacestorage "github.com/anytypeio/any-sync/commonspace/spacestorage"
	"github.com/anytypeio/any-sync/commonspace/syncstatus"
	"github.com/anytypeio/any-sync/net/peer"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"sync/atomic"
	"time"
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
	treeUsage  *atomic.Int32
	isClosed   bool
	isDeleted  bool
}

var log = logger.NewNamed("commonspace.synctree")

var buildObjectTree = objecttree.BuildObjectTree
var createSyncClient = newSyncClient

type ResponsiblePeersGetter interface {
	GetResponsiblePeers(ctx context.Context) (peers []peer.Peer, err error)
}

type BuildDeps struct {
	SpaceId            string
	ObjectSync         objectsync.ObjectSync
	Configuration      nodeconf.Configuration
	HeadNotifiable     HeadNotifiable
	Listener           updatelistener.UpdateListener
	AclList            list.AclList
	SpaceStorage       spacestorage.SpaceStorage
	TreeStorage        treestorage.TreeStorage
	TreeUsage          *atomic.Int32
	SyncStatus         syncstatus.StatusUpdater
	PeerGetter         ResponsiblePeersGetter
	WaitTreeRemoteSync bool
}

func BuildSyncTreeOrGetRemote(ctx context.Context, id string, deps BuildDeps) (t SyncTree, err error) {
	getPeers := func(ctx context.Context) (peerIds []string, err error) {
		peerId, err := peer.CtxPeerId(ctx)
		if err == nil {
			peerIds = []string{peerId}
			return
		}
		err = nil
		log.WarnCtx(ctx, "peer not found in context, use responsible")
		respPeers, err := deps.PeerGetter.GetResponsiblePeers(ctx)
		if err != nil {
			return
		}
		if len(respPeers) == 0 {
			err = fmt.Errorf("no responsible peers")
			return
		}
		for _, p := range respPeers {
			peerIds = append(peerIds, p.Id())
		}
		return
	}

	getTreeRemote := func(peerId string) (msg *treechangeproto.TreeSyncMessage, err error) {
		newTreeRequest := GetRequestFactory().CreateNewTreeRequest()
		objMsg, err := marshallTreeMessage(newTreeRequest, deps.SpaceId, id, "")
		if err != nil {
			return
		}

		resp, err := deps.ObjectSync.MessagePool().SendSync(ctx, peerId, objMsg)
		if err != nil {
			return
		}

		msg = &treechangeproto.TreeSyncMessage{}
		err = proto.Unmarshal(resp.Payload, msg)
		return
	}

	waitTree := func(wait bool) (msg *treechangeproto.TreeSyncMessage, err error) {
		peerIdx := 0
	Loop:
		for {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("waiting for object %s interrupted, context closed", id)
			default:
				break
			}
			availablePeers, err := getPeers(ctx)
			if err != nil {
				if !wait {
					return nil, err
				}
				select {
				// wait for peers to connect
				case <-time.After(1 * time.Second):
					continue Loop
				case <-ctx.Done():
					return nil, fmt.Errorf("waiting for object %s interrupted, context closed", id)
				}
			}

			peerIdx = peerIdx % len(availablePeers)
			msg, err = getTreeRemote(availablePeers[peerIdx])
			if err == nil || !wait {
				return msg, err
			}
			peerIdx++
		}
	}

	deps.TreeStorage, err = deps.SpaceStorage.TreeStorage(id)
	if err == nil {
		return buildSyncTree(ctx, false, deps)
	}

	if err != nil && err != treestorage.ErrUnknownTreeId {
		return
	}

	status, err := deps.SpaceStorage.TreeDeletedStatus(id)
	if err != nil {
		return
	}
	if status != "" {
		err = spacestorage.ErrTreeStorageAlreadyDeleted
		return
	}

	resp, err := waitTree(deps.WaitTreeRemoteSync)
	if err != nil {
		return
	}
	if resp.GetContent().GetFullSyncResponse() == nil {
		err = fmt.Errorf("expected to get full sync response, but got something else")
		return
	}
	fullSyncResp := resp.GetContent().GetFullSyncResponse()

	payload := treestorage.TreeStorageCreatePayload{
		RootRawChange: resp.RootChange,
		Changes:       fullSyncResp.Changes,
		Heads:         fullSyncResp.Heads,
	}

	// basically building tree with in-memory storage and validating that it was without errors
	log.With(zap.String("id", id)).DebugCtx(ctx, "validating tree")
	err = objecttree.ValidateRawTree(payload, deps.AclList)
	if err != nil {
		return
	}
	// now we are sure that we can save it to the storage
	deps.TreeStorage, err = deps.SpaceStorage.CreateTreeStorage(payload)
	if err != nil {
		return
	}
	return buildSyncTree(ctx, true, deps)
}

func PutSyncTree(ctx context.Context, payload treestorage.TreeStorageCreatePayload, deps BuildDeps) (t SyncTree, err error) {
	deps.TreeStorage, err = deps.SpaceStorage.CreateTreeStorage(payload)
	if err != nil {
		return
	}
	return buildSyncTree(ctx, true, deps)
}

func buildSyncTree(ctx context.Context, isFirstBuild bool, deps BuildDeps) (t SyncTree, err error) {
	objTree, err := buildObjectTree(deps.TreeStorage, deps.AclList)
	if err != nil {
		return
	}
	syncClient := createSyncClient(
		deps.SpaceId,
		deps.ObjectSync.MessagePool(),
		sharedFactory,
		deps.Configuration)
	syncTree := &syncTree{
		ObjectTree: objTree,
		syncClient: syncClient,
		notifiable: deps.HeadNotifiable,
		treeUsage:  deps.TreeUsage,
		listener:   deps.Listener,
		syncStatus: deps.SyncStatus,
	}
	syncHandler := newSyncTreeHandler(syncTree, syncClient, deps.SyncStatus)
	syncTree.SyncHandler = syncHandler
	t = syncTree
	syncTree.Lock()
	syncTree.afterBuild()
	syncTree.Unlock()

	if isFirstBuild {
		headUpdate := syncTree.syncClient.CreateHeadUpdate(t, nil)
		// send to everybody, because everybody should know that the node or client got new tree
		if e := syncTree.syncClient.Broadcast(ctx, headUpdate); e != nil {
			log.ErrorCtx(ctx, "broadcast error", zap.Error(e))
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
	err = s.syncClient.Broadcast(ctx, headUpdate)
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
		err = s.syncClient.Broadcast(ctx, headUpdate)
	}
	return
}

func (s *syncTree) Delete() (err error) {
	log.With(zap.String("id", s.Id())).Debug("deleting sync tree")
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

func (s *syncTree) Close() (err error) {
	log.With(zap.String("id", s.Id())).Debug("closing sync tree")
	s.Lock()
	defer s.Unlock()
	if s.isClosed {
		return ErrSyncTreeClosed
	}
	s.treeUsage.Add(-1)
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
	return s.syncClient.SendWithReply(ctx, peerId, headUpdate, "")
}

func (s *syncTree) afterBuild() {
	if s.listener != nil {
		s.listener.Rebuild(s)
	}
	s.treeUsage.Add(1)
	if s.notifiable != nil {
		s.notifiable.UpdateHeads(s.Id(), s.Heads())
	}
}

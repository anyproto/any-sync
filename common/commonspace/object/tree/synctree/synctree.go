package synctree

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/objecttree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/objectsync"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/objectsync/synchandler"
	spacestorage "github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncstatus"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"sync/atomic"
)

var (
	ErrSyncTreeClosed  = errors.New("sync tree is closed")
	ErrSyncTreeDeleted = errors.New("sync tree is deleted")
)

type HeadNotifiable interface {
	UpdateHeads(id string, heads []string)
}

type SyncTree interface {
	objecttree.ObjectTree
	synchandler.SyncHandler
	Ping() (err error)
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

var log = logger.NewNamed("commonspace.synctree").Sugar()

var buildObjectTree = objecttree.BuildObjectTree
var createSyncClient = newWrappedSyncClient

type BuildDeps struct {
	SpaceId        string
	ObjectSync     objectsync.ObjectSync
	Configuration  nodeconf.Configuration
	HeadNotifiable HeadNotifiable
	Listener       updatelistener.UpdateListener
	AclList        list.AclList
	SpaceStorage   spacestorage.SpaceStorage
	TreeStorage    treestorage.TreeStorage
	TreeUsage      *atomic.Int32
	SyncStatus     syncstatus.StatusUpdater
}

func newWrappedSyncClient(
	spaceId string,
	factory RequestFactory,
	objectSync objectsync.ObjectSync,
	configuration nodeconf.Configuration) SyncClient {
	syncClient := newSyncClient(spaceId, objectSync.StreamPool(), factory, configuration, objectSync.StreamChecker())
	return newQueuedClient(syncClient, objectSync.ActionQueue())
}

func BuildSyncTreeOrGetRemote(ctx context.Context, id string, deps BuildDeps) (t SyncTree, err error) {
	getTreeRemote := func() (msg *treechangeproto.TreeSyncMessage, err error) {
		peerId, err := peer.CtxPeerId(ctx)
		if err != nil {
			return
		}

		newTreeRequest := GetRequestFactory().CreateNewTreeRequest()
		objMsg, err := marshallTreeMessage(newTreeRequest, id, "")
		if err != nil {
			return
		}

		err = deps.ObjectSync.StreamChecker().CheckPeerConnection(peerId)
		if err != nil {
			return
		}

		resp, err := deps.ObjectSync.StreamPool().SendSync(peerId, objMsg)
		if err != nil {
			return
		}

		msg = &treechangeproto.TreeSyncMessage{}
		err = proto.Unmarshal(resp.Payload, msg)
		return
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

	resp, err := getTreeRemote()
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
	log.With(zap.String("id", id)).Debug("validating tree")
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
		sharedFactory,
		deps.ObjectSync,
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
		err = syncTree.syncClient.BroadcastAsync(headUpdate)
	}
	return
}

func (s *syncTree) IterateFrom(id string, convert objecttree.ChangeConvertFunc, iterate objecttree.ChangeIterateFunc) (err error) {
	if err = s.checkAlive(); err != nil {
		return
	}
	return s.ObjectTree.IterateFrom(id, convert, iterate)
}

func (s *syncTree) Iterate(convert objecttree.ChangeConvertFunc, iterate objecttree.ChangeIterateFunc) (err error) {
	if err = s.checkAlive(); err != nil {
		return
	}
	return s.ObjectTree.Iterate(convert, iterate)
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
	err = s.syncClient.BroadcastAsync(headUpdate)
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
		err = s.syncClient.BroadcastAsync(headUpdate)
	}
	return
}

func (s *syncTree) Delete() (err error) {
	log.With("id", s.Id()).Debug("deleting sync tree")
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
	log.With("id", s.Id()).Debug("closing sync tree")
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

func (s *syncTree) Ping() (err error) {
	s.Lock()
	defer s.Unlock()
	headUpdate := s.syncClient.CreateHeadUpdate(s, nil)
	return s.syncClient.BroadcastAsyncOrSendResponsible(headUpdate)
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

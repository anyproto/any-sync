package synctree

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/diffservice"
	spacestorage "github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice/synchandler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
)

var ErrSyncTreeClosed = errors.New("sync tree is closed")

// SyncTree sends head updates to sync service and also sends new changes to update listener
type SyncTree struct {
	tree.ObjectTree
	synchandler.SyncHandler
	syncClient SyncClient
	listener   updatelistener.UpdateListener
	isClosed   bool
}

var log = logger.NewNamed("commonspace.synctree").Sugar()

var createDerivedObjectTree = tree.CreateDerivedObjectTree
var createObjectTree = tree.CreateObjectTree
var buildObjectTree = tree.BuildObjectTree
var createSyncClient = newSyncClient

type CreateDeps struct {
	SpaceId        string
	Payload        tree.ObjectTreeCreatePayload
	Configuration  nodeconf.Configuration
	HeadNotifiable diffservice.HeadNotifiable
	StreamPool     syncservice.StreamPool
	Listener       updatelistener.UpdateListener
	AclList        list.ACLList
	CreateStorage  storage.TreeStorageCreatorFunc
}

type BuildDeps struct {
	SpaceId        string
	StreamPool     syncservice.StreamPool
	Configuration  nodeconf.Configuration
	HeadNotifiable diffservice.HeadNotifiable
	Listener       updatelistener.UpdateListener
	AclList        list.ACLList
	SpaceStorage   spacestorage.SpaceStorage
	TreeStorage    storage.TreeStorage
}

func DeriveSyncTree(ctx context.Context, deps CreateDeps) (t tree.ObjectTree, err error) {
	t, err = createDerivedObjectTree(deps.Payload, deps.AclList, deps.CreateStorage)
	if err != nil {
		return
	}
	syncClient := createSyncClient(
		deps.SpaceId,
		deps.StreamPool,
		deps.HeadNotifiable,
		sharedFactory,
		deps.Configuration)
	syncTree := &SyncTree{
		ObjectTree: t,
		syncClient: syncClient,
		listener:   deps.Listener,
	}
	syncHandler := newSyncTreeHandler(syncTree, syncClient)
	syncTree.SyncHandler = syncHandler
	t = syncTree

	headUpdate := syncClient.CreateHeadUpdate(t, nil)
	err = syncClient.BroadcastAsync(headUpdate)
	return
}

func CreateSyncTree(ctx context.Context, deps CreateDeps) (t tree.ObjectTree, err error) {
	t, err = createObjectTree(deps.Payload, deps.AclList, deps.CreateStorage)
	if err != nil {
		return
	}
	syncClient := createSyncClient(
		deps.SpaceId,
		deps.StreamPool,
		deps.HeadNotifiable,
		GetRequestFactory(),
		deps.Configuration)
	syncTree := &SyncTree{
		ObjectTree: t,
		syncClient: syncClient,
		listener:   deps.Listener,
	}
	syncHandler := newSyncTreeHandler(syncTree, syncClient)
	syncTree.SyncHandler = syncHandler
	t = syncTree

	headUpdate := syncClient.CreateHeadUpdate(t, nil)
	err = syncClient.BroadcastAsync(headUpdate)
	return
}

func BuildSyncTreeOrGetRemote(ctx context.Context, id string, deps BuildDeps) (t tree.ObjectTree, err error) {
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

		resp, err := deps.StreamPool.SendSync(peerId, objMsg)
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

	if err != nil && err != storage.ErrUnknownTreeId {
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

	payload := storage.TreeStorageCreatePayload{
		RootRawChange: resp.RootChange,
		Changes:       fullSyncResp.Changes,
		Heads:         fullSyncResp.Heads,
	}

	// basically building tree with in-memory storage and validating that it was without errors
	log.With(zap.String("id", id)).Debug("validating tree")
	err = tree.ValidateRawTree(payload, deps.AclList)
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

func buildSyncTree(ctx context.Context, isFirstBuild bool, deps BuildDeps) (t tree.ObjectTree, err error) {

	t, err = buildObjectTree(deps.TreeStorage, deps.AclList)
	if err != nil {
		return
	}
	syncClient := createSyncClient(
		deps.SpaceId,
		deps.StreamPool,
		deps.HeadNotifiable,
		GetRequestFactory(),
		deps.Configuration)
	syncTree := &SyncTree{
		ObjectTree: t,
		syncClient: syncClient,
		listener:   deps.Listener,
	}
	syncHandler := newSyncTreeHandler(syncTree, syncClient)
	syncTree.SyncHandler = syncHandler
	t = syncTree

	headUpdate := syncTree.syncClient.CreateHeadUpdate(t, nil)
	// here we will have different behaviour based on who is sending this update
	if isFirstBuild {
		// send to everybody, because everybody should know that the node or client got new tree
		err = syncTree.syncClient.BroadcastAsync(headUpdate)
	} else {
		// send either to everybody if client or to replica set if node
		err = syncTree.syncClient.BroadcastAsyncOrSendResponsible(headUpdate)
	}
	return
}

func (s *SyncTree) AddContent(ctx context.Context, content tree.SignableChangeContent) (res tree.AddResult, err error) {
	if s.isClosed {
		err = ErrSyncTreeClosed
		return
	}
	res, err = s.ObjectTree.AddContent(ctx, content)
	if err != nil {
		return
	}
	headUpdate := s.syncClient.CreateHeadUpdate(s, res.Added)
	err = s.syncClient.BroadcastAsync(headUpdate)
	return
}

func (s *SyncTree) AddRawChanges(ctx context.Context, changes ...*treechangeproto.RawTreeChangeWithId) (res tree.AddResult, err error) {
	if s.isClosed {
		err = ErrSyncTreeClosed
		return
	}
	res, err = s.ObjectTree.AddRawChanges(ctx, changes...)
	if err != nil {
		return
	}
	if s.listener != nil {
		switch res.Mode {
		case tree.Nothing:
			return
		case tree.Append:
			s.listener.Update(s)
		case tree.Rebuild:
			s.listener.Rebuild(s)
		}
	}
	//if res.Mode != tree.Nothing {
	headUpdate := s.syncClient.CreateHeadUpdate(s, res.Added)
	err = s.syncClient.BroadcastAsync(headUpdate)
	//}
	return
}

func (s *SyncTree) Close() (err error) {
	log.With("id", s.ID()).Debug("closing sync tree")
	s.Lock()
	defer s.Unlock()
	log.With("id", s.ID()).Debug("taken lock on sync tree")
	if s.isClosed {
		err = ErrSyncTreeClosed
		return
	}
	s.isClosed = true
	return
}

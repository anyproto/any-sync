package synctree

import (
	"context"
	"errors"

	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/net/peer"
)

var (
	ErrNoResponsiblePeers = errors.New("no responsible peers")
)

type treeRemoteGetter struct {
	deps   BuildDeps
	treeId string
}

func (t treeRemoteGetter) getPeers(ctx context.Context) (peerIds []string, err error) {
	peerId, err := peer.CtxPeerId(ctx)
	if err != nil {
		return nil, err
	}
	if peerId != peer.CtxResponsiblePeers {
		peerIds = []string{peerId}
		return
	}

	log.InfoCtx(ctx, "use responsible peers")
	respPeers, err := t.deps.PeerGetter.GetResponsiblePeers(ctx)
	if err != nil {
		return
	}
	if len(respPeers) == 0 {
		err = ErrNoResponsiblePeers
		return
	}
	for _, p := range respPeers {
		peerIds = append(peerIds, p.Id())
	}
	return
}

func (t treeRemoteGetter) treeRequest(ctx context.Context, peerId string) (msg *treechangeproto.TreeSyncMessage, err error) {
	newTreeRequest := t.deps.SyncClient.CreateNewTreeRequest()
	resp, err := t.deps.SyncClient.SendRequest(ctx, peerId, t.treeId, newTreeRequest)
	if err != nil {
		return
	}

	msg = &treechangeproto.TreeSyncMessage{}
	err = proto.Unmarshal(resp.Payload, msg)
	return
}

func (t treeRemoteGetter) treeRequestLoop(ctx context.Context) (msg *treechangeproto.TreeSyncMessage, err error) {
	availablePeers, err := t.getPeers(ctx)
	if err != nil {
		return
	}
	// in future we will try to load from different peers
	return t.treeRequest(ctx, availablePeers[0])
}

func (t treeRemoteGetter) getTree(ctx context.Context) (treeStorage treestorage.TreeStorage, isRemote bool, err error) {
	treeStorage, err = t.deps.SpaceStorage.TreeStorage(t.treeId)
	if err == nil {
		return
	}
	if err != nil && err != treestorage.ErrUnknownTreeId {
		return
	}

	status, err := t.deps.SpaceStorage.TreeDeletedStatus(t.treeId)
	if err != nil {
		return
	}
	if status != "" {
		err = spacestorage.ErrTreeStorageAlreadyDeleted
		return
	}

	isRemote = true
	resp, err := t.treeRequestLoop(ctx)
	if err != nil {
		return
	}
	fullSyncResp := resp.GetContent().GetFullSyncResponse()
	if fullSyncResp == nil {
		err = treechangeproto.ErrUnexpected
		return
	}

	payload := treestorage.TreeStorageCreatePayload{
		RootRawChange: resp.RootChange,
		Changes:       fullSyncResp.Changes,
		Heads:         fullSyncResp.Heads,
	}

	validatorFunc := t.deps.ValidateObjectTree
	if validatorFunc == nil {
		validatorFunc = objecttree.ValidateRawTreeBuildFunc
	}
	// basically building tree with in-memory storage and validating that it was without errors
	log.With(zap.String("id", t.treeId)).DebugCtx(ctx, "validating tree")
	newPayload, err := validatorFunc(payload, t.deps.BuildObjectTree, t.deps.AclList)
	if err != nil {
		return
	}
	// now we are sure that we can save it to the storage
	treeStorage, err = t.deps.SpaceStorage.CreateTreeStorage(newPayload)
	return
}

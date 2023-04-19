package synctree

import (
	"context"
	"fmt"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
	"github.com/anytypeio/any-sync/commonspace/objectsync"
	"github.com/anytypeio/any-sync/commonspace/spacestorage"
	"github.com/anytypeio/any-sync/net/peer"
	"github.com/anytypeio/any-sync/net/rpc/rpcerr"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"time"
)

type treeRemoteGetter struct {
	deps   BuildDeps
	treeId string
}

func newRemoteGetter(treeId string, deps BuildDeps) treeRemoteGetter {
	return treeRemoteGetter{treeId: treeId, deps: deps}
}

func (t treeRemoteGetter) getPeers(ctx context.Context) (peerIds []string, err error) {
	peerId, err := peer.CtxPeerId(ctx)
	if err == nil {
		peerIds = []string{peerId}
		return
	}
	err = nil
	log.WarnCtx(ctx, "peer not found in context, use responsible")
	respPeers, err := t.deps.PeerGetter.GetResponsiblePeers(ctx)
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

func (t treeRemoteGetter) treeRequest(ctx context.Context, peerId string) (msg *treechangeproto.TreeSyncMessage, err error) {
	newTreeRequest := objectsync.GetRequestFactory().CreateNewTreeRequest()
	resp, err := t.deps.SyncClient.SendSync(ctx, peerId, t.treeId, newTreeRequest)
	if err != nil {
		return
	}

	msg = &treechangeproto.TreeSyncMessage{}
	err = proto.Unmarshal(resp.Payload, msg)
	return
}

func (t treeRemoteGetter) treeRequestLoop(ctx context.Context, wait bool) (msg *treechangeproto.TreeSyncMessage, err error) {
	peerIdx := 0
Loop:
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for object %s interrupted, context closed", t.treeId)
		default:
			break
		}
		availablePeers, err := t.getPeers(ctx)
		if err != nil {
			if !wait {
				return nil, err
			}
			select {
			// wait for peers to connect
			case <-time.After(1 * time.Second):
				continue Loop
			case <-ctx.Done():
				return nil, fmt.Errorf("waiting for object %s interrupted, context closed", t.treeId)
			}
		}

		peerIdx = peerIdx % len(availablePeers)
		msg, err = t.treeRequest(ctx, availablePeers[peerIdx])
		if err == nil || !wait {
			return msg, err
		}
		peerIdx++
	}
}

func (t treeRemoteGetter) getTree(ctx context.Context) (treeStorage treestorage.TreeStorage, err error) {
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

	resp, err := t.treeRequestLoop(ctx, t.deps.WaitTreeRemoteSync)
	if err != nil {
		return
	}
	switch {
	case resp.GetContent().GetErrorResponse() != nil:
		errResp := resp.GetContent().GetErrorResponse()
		err = rpcerr.Err(errResp.ErrCode)
		return
	case resp.GetContent().GetFullSyncResponse() == nil:
		err = fmt.Errorf("expected to get full sync response, but got something else")
		return
	default:
		break
	}
	fullSyncResp := resp.GetContent().GetFullSyncResponse()

	payload := treestorage.TreeStorageCreatePayload{
		RootRawChange: resp.RootChange,
		Changes:       fullSyncResp.Changes,
		Heads:         fullSyncResp.Heads,
	}

	// basically building tree with in-memory storage and validating that it was without errors
	log.With(zap.String("id", t.treeId)).DebugCtx(ctx, "validating tree")
	err = objecttree.ValidateRawTree(payload, t.deps.AclList)
	if err != nil {
		return
	}
	// now we are sure that we can save it to the storage
	return t.deps.SpaceStorage.CreateTreeStorage(payload)
}

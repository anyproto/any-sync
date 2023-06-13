package synctree

import (
	"context"
	"errors"
	"time"

	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
)

var (
	newRequestTimeout = 1 * time.Second

	ErrRetryTimeout       = errors.New("failed to retry request")
	ErrNoResponsiblePeers = errors.New("no responsible peers")
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

func (t treeRemoteGetter) treeRequestLoop(ctx context.Context, retryTimeout time.Duration) (msg *treechangeproto.TreeSyncMessage, err error) {
	peerIdx := 0
	retryCtx, cancel := context.WithTimeout(ctx, retryTimeout)
	defer cancel()
	for {
		availablePeers, err := t.getPeers(ctx)
		if err != nil {
			if retryTimeout == 0 {
				return nil, err
			}
		} else {
			peerIdx = peerIdx % len(availablePeers)
			msg, err = t.treeRequest(ctx, availablePeers[peerIdx])
			if err == nil || retryTimeout == 0 {
				return msg, err
			}
			peerIdx++
		}
		select {
		case <-time.After(newRequestTimeout):
			break
		case <-retryCtx.Done():
			return nil, ErrRetryTimeout
		}
	}
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
	resp, err := t.treeRequestLoop(ctx, t.deps.RetryTimeout)
	if err != nil {
		return
	}
	switch {
	case resp.GetContent().GetErrorResponse() != nil:
		errResp := resp.GetContent().GetErrorResponse()
		err = rpcerr.Err(errResp.ErrCode)
		return
	case resp.GetContent().GetFullSyncResponse() == nil:
		err = treechangeproto.ErrUnexpected
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
	treeStorage, err = t.deps.SpaceStorage.CreateTreeStorage(payload)
	return
}

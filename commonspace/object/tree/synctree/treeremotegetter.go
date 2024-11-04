package synctree

import (
	"context"
	"errors"

	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/net/peer"
)

var (
	ErrNoResponsiblePeers = errors.New("no responsible peers")
)

type treeGetter interface {
	getTree(ctx context.Context) (treeStorage treestorage.TreeStorage, peerId string, err error)
}

type treeRemoteGetter struct {
	deps   BuildDeps
	treeId string
}

var createCollector = newFullResponseCollector

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

func (t treeRemoteGetter) treeRequest(ctx context.Context, peerId string) (collector *fullResponseCollector, err error) {
	collector = createCollector(t.deps)
	req := t.deps.SyncClient.CreateNewTreeRequest(peerId, t.treeId)
	err = t.deps.SyncClient.SendTreeRequest(ctx, req, collector)
	if err != nil {
		return nil, err
	}
	return collector, nil
}

func (t treeRemoteGetter) treeRequestLoop(ctx context.Context) (collector *fullResponseCollector, peerId string, err error) {
	availablePeers, err := t.getPeers(ctx)
	if err != nil {
		return
	}
	peerId = availablePeers[0]
	// in future we will try to load from different peers
	collector, err = t.treeRequest(ctx, peerId)
	return collector, peerId, err
}

func (t treeRemoteGetter) getTree(ctx context.Context) (treeStorage treestorage.TreeStorage, peerId string, err error) {
	treeStorage, err = t.deps.SpaceStorage.TreeStorage(t.treeId)
	if err == nil || !errors.Is(err, treestorage.ErrUnknownTreeId) {
		return
	}
	err = checkTreeDeleted(t.treeId, t.deps.SpaceStorage)
	if err != nil {
		return
	}

	collector, peerId, err := t.treeRequestLoop(ctx)
	if err != nil {
		if errors.Is(err, peer.ErrPeerIdNotFoundInContext) {
			err = treestorage.ErrUnknownTreeId
		}
		return
	}

	return collector.objectTree.Storage(), peerId, nil
}

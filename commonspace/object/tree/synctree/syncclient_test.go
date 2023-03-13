package synctree

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/objectsync/synchandler"
	"github.com/anytypeio/any-sync/commonspace/peermanager"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/commonspace/syncstatus"
	"github.com/anytypeio/any-sync/net/peer"
)

type mockPeerManager struct {
}

func (m *mockPeerManager) SendPeer(ctx context.Context, peerId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockPeerManager) Broadcast(ctx context.Context, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockPeerManager) GetResponsiblePeers(ctx context.Context) (peers []peer.Peer, err error) {
	//TODO implement me
	panic("implement me")
}

type broadcastTree struct {
	objecttree.ObjectTree
	SyncClient
}

func (b *broadcastTree) AddRawChanges(ctx context.Context, changes objecttree.RawChangesPayload) (objecttree.AddResult, error) {
	res, err := b.ObjectTree.AddRawChanges(ctx, changes)
	if err != nil {
		return objecttree.AddResult{}, err
	}
	upd := b.SyncClient.CreateHeadUpdate(b.ObjectTree, res.Added)
	b.SyncClient.Broadcast(ctx, upd)
	return res, nil
}

func build(spaceId string, objTree objecttree.ObjectTree, peerManager peermanager.PeerManager) synchandler.SyncHandler {
	factory := GetRequestFactory()
	syncClient := newSyncClient(spaceId, peerManager, factory)
	netTree := &broadcastTree{
		ObjectTree: objTree,
		SyncClient: syncClient,
	}
	return newSyncTreeHandler(spaceId, netTree, syncClient, syncstatus.NewNoOpSyncStatus())
}

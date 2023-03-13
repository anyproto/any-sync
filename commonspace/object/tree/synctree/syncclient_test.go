package synctree

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/object/acl/list"
	"github.com/anytypeio/any-sync/commonspace/object/acl/testutils/acllistbuilder"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
	"github.com/anytypeio/any-sync/commonspace/objectsync/synchandler"
	"github.com/anytypeio/any-sync/commonspace/peermanager"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/commonspace/syncstatus"
	"github.com/anytypeio/any-sync/net/peer"
	"github.com/stretchr/testify/require"
	"testing"
)

type mockPeerManager struct {
}

func (m *mockPeerManager) SendPeer(ctx context.Context, peerId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	panic("implement me")
}

func (m *mockPeerManager) Broadcast(ctx context.Context, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	panic("implement me")
}

func (m *mockPeerManager) GetResponsiblePeers(ctx context.Context) (peers []peer.Peer, err error) {
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

func createSyncHandler(spaceId string, objTree objecttree.ObjectTree, peerManager peermanager.PeerManager) synchandler.SyncHandler {
	factory := GetRequestFactory()
	syncClient := newSyncClient(spaceId, peerManager, factory)
	netTree := &broadcastTree{
		ObjectTree: objTree,
		SyncClient: syncClient,
	}
	return newSyncTreeHandler(spaceId, netTree, syncClient, syncstatus.NewNoOpSyncStatus())
}

func createAclList() (list.AclList, error) {
	st, err := acllistbuilder.NewListStorageWithTestName("userjoinexample.yml")
	if err != nil {
		return nil, err
	}
	return list.BuildAclList(st)
}

func createStorage(treeId string, aclList list.AclList) treestorage.TreeStorage {
	changeCreator := objecttree.NewMockChangeCreator()
	st := changeCreator.CreateNewTreeStorage(treeId, aclList.Head().Id)
	return st
}

func createTestTree(aclList list.AclList, storage treestorage.TreeStorage) (objecttree.ObjectTree, error) {
	return objecttree.BuildTestableTree(aclList, storage)
}

func TestSyncProtocol(t *testing.T) {
	aclList, err := createAclList()
	require.NoError(t, err)
	treeId := "treeId"
	spaceId := "spaceId"
	storage := createStorage(treeId, aclList)
	testTree, err := createTestTree(aclList, storage)
	require.NoError(t, err)
	peerManager := &mockPeerManager{}
	_ = createSyncHandler(spaceId, testTree, peerManager)
}

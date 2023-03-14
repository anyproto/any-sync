package synctree

import (
	"context"
	"fmt"
	"github.com/anytypeio/any-sync/commonspace/object/acl/list"
	"github.com/anytypeio/any-sync/commonspace/object/acl/testutils/acllistbuilder"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
	"github.com/anytypeio/any-sync/commonspace/objectsync/synchandler"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/commonspace/syncstatus"
	"github.com/anytypeio/any-sync/net/peer"
	"github.com/cheggaaa/mb/v3"
	"sync"
)

type processMsg struct {
	msg        *spacesyncproto.ObjectSyncMessage
	senderId   string
	receiverId string
}

type messageLog struct {
	batcher *mb.MB[processMsg]
}

func newMessageLog() *messageLog {
	return &messageLog{batcher: mb.New[processMsg](0)}
}

func (m *messageLog) addMessage(msg processMsg) {
	m.batcher.Add(context.Background(), msg)
}

type processSyncHandler struct {
	synchandler.SyncHandler
	batcher *mb.MB[processMsg]
	peerId  string
}

func newProcessSyncHandler(peerId string, syncHandler synchandler.SyncHandler) *processSyncHandler {
	batcher := mb.New[processMsg](0)
	return &processSyncHandler{syncHandler, batcher, peerId}
}

func (p *processSyncHandler) manager() *processPeerManager {
	return p.SyncHandler.(*syncTreeHandler).syncClient.(*syncClient).PeerManager.(*processPeerManager)
}

func (p *processSyncHandler) send(ctx context.Context, msg processMsg) (err error) {
	return p.batcher.Add(ctx, msg)
}

func (p *processSyncHandler) run(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			res, err := p.batcher.WaitOne(ctx)
			if err != nil {
				return
			}
			err = p.SyncHandler.HandleMessage(ctx, res.senderId, res.msg)
			if err != nil {
				fmt.Println("error handling message", err.Error())
				continue
			}
		}
	}()
}

type processPeerManager struct {
	peerId   string
	handlers map[string]*processSyncHandler
	log      *messageLog
}

func newProcessPeerManager(peerId string, log *messageLog) *processPeerManager {
	return &processPeerManager{handlers: map[string]*processSyncHandler{}, peerId: peerId, log: log}
}

func (m *processPeerManager) addHandler(peerId string, handler *processSyncHandler) {
	m.handlers[peerId] = handler
}

func (m *processPeerManager) SendPeer(ctx context.Context, peerId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	pMsg := processMsg{
		msg:        msg,
		senderId:   m.peerId,
		receiverId: peerId,
	}
	m.log.addMessage(pMsg)
	return m.handlers[peerId].send(context.Background(), pMsg)
}

func (m *processPeerManager) Broadcast(ctx context.Context, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	for _, handler := range m.handlers {
		pMsg := processMsg{
			msg:        msg,
			senderId:   m.peerId,
			receiverId: handler.peerId,
		}
		m.log.addMessage(pMsg)
		handler.send(context.Background(), pMsg)
	}
	return
}

func (m *processPeerManager) GetResponsiblePeers(ctx context.Context) (peers []peer.Peer, err error) {
	panic("should not be called")
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

func createSyncHandler(peerId, spaceId string, objTree objecttree.ObjectTree, log *messageLog) *processSyncHandler {
	factory := GetRequestFactory()
	syncClient := newSyncClient(spaceId, newProcessPeerManager(peerId, log), factory)
	netTree := &broadcastTree{
		ObjectTree: objTree,
		SyncClient: syncClient,
	}
	handler := newSyncTreeHandler(spaceId, netTree, syncClient, syncstatus.NewNoOpSyncStatus())
	return newProcessSyncHandler(peerId, handler)
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

type processFixture struct {
	handlers []*processSyncHandler
	log      *messageLog
	wg       *sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

//func TestSyncProtocol(t *testing.T) {
//	aclList, err := createAclList()
//	require.NoError(t, err)
//	treeId := "treeId"
//	spaceId := "spaceId"
//	storage := createStorage(treeId, aclList)
//	testTree, err := createTestTree(aclList, storage)
//	require.NoError(t, err)
//	peerManager := &processPeerManager{}
//	_ = createSyncHandler(spaceId, testTree, peerManager)
//}

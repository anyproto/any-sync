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
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

type processMsg struct {
	msg        *spacesyncproto.ObjectSyncMessage
	senderId   string
	receiverId string
	userMsg    *objecttree.RawChangesPayload
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

func (p *processSyncHandler) tree() *broadcastTree {
	return p.SyncHandler.(*syncTreeHandler).objTree.(*broadcastTree)
}

func (p *processSyncHandler) send(ctx context.Context, msg processMsg) (err error) {
	return p.batcher.Add(ctx, msg)
}

func (p *processSyncHandler) sendRawChanges(ctx context.Context, changes objecttree.RawChangesPayload) {
	p.batcher.Add(ctx, processMsg{userMsg: &changes})
}

func (p *processSyncHandler) run(ctx context.Context, t *testing.T, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			res, err := p.batcher.WaitOne(ctx)
			if err != nil {
				return
			}
			if res.userMsg != nil {
				p.tree().Lock()
				userRes, err := p.tree().AddRawChanges(ctx, *res.userMsg)
				require.NoError(t, err)
				fmt.Println("user add result", userRes.Heads)
				p.tree().Unlock()
				continue
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

type fixtureDeps struct {
	aclList       list.AclList
	initStorage   *treestorage.InMemoryTreeStorage
	connectionMap map[string][]string
}

type processFixture struct {
	handlers map[string]*processSyncHandler
	log      *messageLog
	wg       *sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

func newProcessFixture(t *testing.T, spaceId string, deps fixtureDeps) *processFixture {
	var (
		handlers    = map[string]*processSyncHandler{}
		log         = newMessageLog()
		wg          = sync.WaitGroup{}
		ctx, cancel = context.WithCancel(context.Background())
	)

	for peerId := range deps.connectionMap {
		stCopy := deps.initStorage.Copy()
		testTree, err := createTestTree(deps.aclList, stCopy)
		require.NoError(t, err)
		handler := createSyncHandler(peerId, spaceId, testTree, log)
		handlers[peerId] = handler
	}
	for peerId, connectionMap := range deps.connectionMap {
		handler := handlers[peerId]
		manager := handler.manager()
		for _, connectionId := range connectionMap {
			manager.addHandler(connectionId, handlers[connectionId])
		}
	}
	return &processFixture{
		handlers: handlers,
		log:      log,
		wg:       &wg,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (p *processFixture) runProcessFixture(t *testing.T) {
	for _, handler := range p.handlers {
		handler.run(p.ctx, t, p.wg)
	}
}

func (p *processFixture) stop() {
	p.cancel()
	p.wg.Wait()
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

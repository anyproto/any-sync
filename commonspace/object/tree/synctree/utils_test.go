package synctree

import (
	"context"
	"fmt"
	"github.com/anytypeio/any-sync/commonspace/object/acl/list"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
	"github.com/anytypeio/any-sync/commonspace/objectsync/synchandler"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/commonspace/syncstatus"
	"github.com/anytypeio/any-sync/net/peer"
	"github.com/cheggaaa/mb/v3"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
	"math/rand"
	"sync"
	"testing"
	"time"
)

type processMsg struct {
	msg        *spacesyncproto.ObjectSyncMessage
	senderId   string
	receiverId string
	userMsg    *objecttree.RawChangesPayload
}

type msgDescription struct {
	name    string
	from    string
	to      string
	heads   []string
	changes []*treechangeproto.RawTreeChangeWithId
}

func (p *processMsg) description() (descr msgDescription) {
	unmarshalled := &treechangeproto.TreeSyncMessage{}
	err := proto.Unmarshal(p.msg.Payload, unmarshalled)
	if err != nil {
		panic(err)
	}
	descr = msgDescription{
		from: p.senderId,
		to:   p.receiverId,
	}
	switch {
	case unmarshalled.GetContent().GetHeadUpdate() != nil:
		cnt := unmarshalled.GetContent().GetHeadUpdate()
		descr.name = "HeadUpdate"
		descr.heads = cnt.Heads
		descr.changes = unmarshalled.GetContent().GetHeadUpdate().Changes
	case unmarshalled.GetContent().GetFullSyncRequest() != nil:
		cnt := unmarshalled.GetContent().GetFullSyncRequest()
		descr.name = "FullSyncRequest"
		descr.heads = cnt.Heads
		descr.changes = unmarshalled.GetContent().GetFullSyncRequest().Changes
	case unmarshalled.GetContent().GetFullSyncResponse() != nil:
		cnt := unmarshalled.GetContent().GetFullSyncResponse()
		descr.name = "FullSyncResponse"
		descr.heads = cnt.Heads
		descr.changes = unmarshalled.GetContent().GetFullSyncResponse().Changes
	}
	return
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
	batcher    *mb.MB[processMsg]
	peerId     string
	aclList    list.AclList
	log        *messageLog
	syncClient SyncClient
}

func (p *processSyncHandler) HandleMessage(ctx context.Context, senderId string, request *spacesyncproto.ObjectSyncMessage) (err error) {
	if p.SyncHandler != nil {
		return p.SyncHandler.HandleMessage(ctx, senderId, request)
	}
	unmarshalled := &treechangeproto.TreeSyncMessage{}
	err = proto.Unmarshal(request.Payload, unmarshalled)
	if err != nil {
		return
	}
	if unmarshalled.Content.GetFullSyncResponse() == nil {
		newTreeRequest := GetRequestFactory().CreateNewTreeRequest()
		var objMsg *spacesyncproto.ObjectSyncMessage
		objMsg, err = marshallTreeMessage(newTreeRequest, request.SpaceId, request.ObjectId, "")
		if err != nil {
			return
		}
		return p.manager().SendPeer(context.Background(), senderId, objMsg)
	}
	fullSyncResponse := unmarshalled.Content.GetFullSyncResponse()
	treeStorage, _ := treestorage.NewInMemoryTreeStorage(unmarshalled.RootChange, []string{unmarshalled.RootChange.Id}, nil)
	tree, err := createTestTree(p.aclList, treeStorage)
	if err != nil {
		return
	}
	netTree := &broadcastTree{
		ObjectTree: tree,
		SyncClient: p.syncClient,
	}
	res, err := netTree.AddRawChanges(context.Background(), objecttree.RawChangesPayload{
		NewHeads:   fullSyncResponse.Heads,
		RawChanges: fullSyncResponse.Changes,
	})
	if err != nil {
		return
	}
	p.SyncHandler = newSyncTreeHandler(request.SpaceId, netTree, p.syncClient, syncstatus.NewNoOpSyncStatus())
	var objMsg *spacesyncproto.ObjectSyncMessage
	newTreeRequest := GetRequestFactory().CreateHeadUpdate(netTree, res.Added)
	objMsg, err = marshallTreeMessage(newTreeRequest, request.SpaceId, request.ObjectId, "")
	if err != nil {
		return
	}
	return p.manager().Broadcast(context.Background(), objMsg)
}

func newProcessSyncHandler(peerId string, syncHandler synchandler.SyncHandler) *processSyncHandler {
	batcher := mb.New[processMsg](0)
	return &processSyncHandler{
		SyncHandler: syncHandler,
		batcher:     batcher,
		peerId:      peerId,
	}
}

func (p *processSyncHandler) manager() *processPeerManager {
	if p.SyncHandler != nil {
		return p.SyncHandler.(*syncTreeHandler).syncClient.(*syncClient).PeerManager.(*processPeerManager)
	}
	return p.syncClient.(*syncClient).PeerManager.(*processPeerManager)
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
			err = p.HandleMessage(ctx, res.senderId, res.msg)
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

func createEmptySyncHandler(peerId, spaceId string, aclList list.AclList, log *messageLog) *processSyncHandler {
	factory := GetRequestFactory()
	syncClient := newSyncClient(spaceId, newProcessPeerManager(peerId, log), factory)

	batcher := mb.New[processMsg](0)
	return &processSyncHandler{
		batcher:    batcher,
		peerId:     peerId,
		aclList:    aclList,
		log:        log,
		syncClient: syncClient,
	}
}

func createStorage(treeId string, aclList list.AclList) treestorage.TreeStorage {
	changeCreator := objecttree.NewMockChangeCreator()
	st := changeCreator.CreateNewTreeStorage(treeId, aclList.Head().Id)
	return st
}

func createTestTree(aclList list.AclList, storage treestorage.TreeStorage) (objecttree.ObjectTree, error) {
	return objecttree.BuildEmptyDataTestableTree(aclList, storage)
}

type fixtureDeps struct {
	aclList       list.AclList
	initStorage   *treestorage.InMemoryTreeStorage
	connectionMap map[string][]string
	emptyTrees    []string
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
		var handler *processSyncHandler
		if slices.Contains(deps.emptyTrees, peerId) {
			handler = createEmptySyncHandler(peerId, spaceId, deps.aclList, log)
		} else {
			stCopy := deps.initStorage.Copy()
			testTree, err := createTestTree(deps.aclList, stCopy)
			require.NoError(t, err)
			handler = createSyncHandler(peerId, spaceId, testTree, log)
		}
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

func (p *processFixture) run(t *testing.T) {
	for _, handler := range p.handlers {
		handler.run(p.ctx, t, p.wg)
	}
}

func (p *processFixture) stop() {
	p.cancel()
	p.wg.Wait()
}

type genParams struct {
	prefix     string
	aclId      string
	startIdx   int
	levels     int
	perLevel   int
	snapshotId string
	prevHeads  []string
	isSnapshot func() bool
}

type genResult struct {
	changes    []*treechangeproto.RawTreeChangeWithId
	heads      []string
	snapshotId string
}

func genChanges(creator *objecttree.MockChangeCreator, params genParams) (res genResult) {
	src := rand.NewSource(time.Now().Unix())
	rnd := rand.New(src)
	var (
		prevHeads  []string
		snapshotId = params.snapshotId
	)
	prevHeads = append(prevHeads, params.prevHeads...)

	for i := 0; i < params.levels; i++ {
		if params.isSnapshot() {
			newId := fmt.Sprintf("%s.%d.%d", params.prefix, params.startIdx+i, 0)
			newCh := creator.CreateRaw(newId, params.aclId, snapshotId, true, prevHeads...)
			res.changes = append(res.changes, newCh)
			prevHeads = []string{newId}
			snapshotId = newId
			continue
		}
		perLevel := rnd.Intn(params.perLevel)
		if perLevel == 0 {
			perLevel = 1
		}
		var (
			newHeads []string
			usedIds  = map[string]struct{}{}
		)
		for j := 0; j < perLevel; j++ {
			prevConns := rnd.Intn(len(prevHeads))
			if prevConns == 0 {
				prevConns = 1
			}
			rnd.Shuffle(len(prevHeads), func(i, j int) {
				prevHeads[i], prevHeads[j] = prevHeads[j], prevHeads[i]
			})
			// if we didn't connect with all prev ones
			if j == perLevel-1 && len(usedIds) != len(prevHeads) {
				var unusedIds []string
				for _, id := range prevHeads {
					if _, exists := usedIds[id]; !exists {
						unusedIds = append(unusedIds, id)
					}
				}
				prevHeads = unusedIds
				prevConns = len(prevHeads)
			}
			var prevChId []string
			for k := 0; k < prevConns; k++ {
				prevChId = append(prevChId, prevHeads[k])
				usedIds[prevHeads[k]] = struct{}{}
			}
			newId := fmt.Sprintf("%s.%d.%d", params.prefix, params.startIdx+i, j)
			newCh := creator.CreateRaw(newId, params.aclId, snapshotId, false, prevChId...)
			res.changes = append(res.changes, newCh)
			newHeads = append(newHeads, newId)
		}
		prevHeads = newHeads
	}
	res.heads = prevHeads
	res.snapshotId = snapshotId
	return
}

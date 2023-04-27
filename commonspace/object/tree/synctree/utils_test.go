package synctree

import (
	"context"
	"fmt"
	"github.com/anytypeio/any-sync/commonspace/object/acl/list"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
	"github.com/anytypeio/any-sync/commonspace/objectsync"
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

// protocolMsg is a message used in sync protocol tests
type protocolMsg struct {
	msg        *spacesyncproto.ObjectSyncMessage
	senderId   string
	receiverId string
	userMsg    *objecttree.RawChangesPayload
}

// msgDescription is a representation of message used for checking the results of the test
type msgDescription struct {
	name    string
	from    string
	to      string
	heads   []string
	changes []*treechangeproto.RawTreeChangeWithId
}

func (p *protocolMsg) description() (descr msgDescription) {
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

// messageLog saves all messages that were sent during sync test
type messageLog struct {
	batcher *mb.MB[protocolMsg]
}

func newMessageLog() *messageLog {
	return &messageLog{batcher: mb.New[protocolMsg](0)}
}

func (m *messageLog) addMessage(msg protocolMsg) {
	m.batcher.Add(context.Background(), msg)
}

// testSyncHandler is the wrapper around individual tree to test sync protocol
type testSyncHandler struct {
	synchandler.SyncHandler
	batcher    *mb.MB[protocolMsg]
	peerId     string
	aclList    list.AclList
	log        *messageLog
	syncClient objectsync.SyncClient
}

// createSyncHandler creates a sync handler when a tree is already created
func createSyncHandler(peerId, spaceId string, objTree objecttree.ObjectTree, log *messageLog) *testSyncHandler {
	factory := objectsync.NewRequestFactory()
	syncClient := objectsync.NewSyncClient(spaceId, newTestMessagePool(peerId, log), factory)
	netTree := &broadcastTree{
		ObjectTree: objTree,
		SyncClient: syncClient,
	}
	handler := newSyncTreeHandler(spaceId, netTree, syncClient, syncstatus.NewNoOpSyncStatus())
	return newTestSyncHandler(peerId, handler)
}

// createEmptySyncHandler creates a sync handler when the tree will be provided later (this emulates the situation when we have no tree)
func createEmptySyncHandler(peerId, spaceId string, aclList list.AclList, log *messageLog) *testSyncHandler {
	factory := objectsync.NewRequestFactory()
	syncClient := objectsync.NewSyncClient(spaceId, newTestMessagePool(peerId, log), factory)

	batcher := mb.New[protocolMsg](0)
	return &testSyncHandler{
		batcher:    batcher,
		peerId:     peerId,
		aclList:    aclList,
		log:        log,
		syncClient: syncClient,
	}
}

func newTestSyncHandler(peerId string, syncHandler synchandler.SyncHandler) *testSyncHandler {
	batcher := mb.New[protocolMsg](0)
	return &testSyncHandler{
		SyncHandler: syncHandler,
		batcher:     batcher,
		peerId:      peerId,
	}
}

func (h *testSyncHandler) HandleMessage(ctx context.Context, senderId string, request *spacesyncproto.ObjectSyncMessage) (err error) {
	if h.SyncHandler != nil {
		return h.SyncHandler.HandleMessage(ctx, senderId, request)
	}
	unmarshalled := &treechangeproto.TreeSyncMessage{}
	err = proto.Unmarshal(request.Payload, unmarshalled)
	if err != nil {
		return
	}
	if unmarshalled.Content.GetFullSyncResponse() == nil {
		newTreeRequest := objectsync.NewRequestFactory().CreateNewTreeRequest()
		var objMsg *spacesyncproto.ObjectSyncMessage
		objMsg, err = objectsync.MarshallTreeMessage(newTreeRequest, request.SpaceId, request.ObjectId, "")
		if err != nil {
			return
		}
		return h.manager().SendPeer(context.Background(), senderId, objMsg)
	}
	fullSyncResponse := unmarshalled.Content.GetFullSyncResponse()
	treeStorage, _ := treestorage.NewInMemoryTreeStorage(unmarshalled.RootChange, []string{unmarshalled.RootChange.Id}, nil)
	tree, err := createTestTree(h.aclList, treeStorage)
	if err != nil {
		return
	}
	netTree := &broadcastTree{
		ObjectTree: tree,
		SyncClient: h.syncClient,
	}
	res, err := netTree.AddRawChanges(context.Background(), objecttree.RawChangesPayload{
		NewHeads:   fullSyncResponse.Heads,
		RawChanges: fullSyncResponse.Changes,
	})
	if err != nil {
		return
	}
	h.SyncHandler = newSyncTreeHandler(request.SpaceId, netTree, h.syncClient, syncstatus.NewNoOpSyncStatus())
	var objMsg *spacesyncproto.ObjectSyncMessage
	newTreeRequest := objectsync.NewRequestFactory().CreateHeadUpdate(netTree, res.Added)
	objMsg, err = objectsync.MarshallTreeMessage(newTreeRequest, request.SpaceId, request.ObjectId, "")
	if err != nil {
		return
	}
	return h.manager().Broadcast(context.Background(), objMsg)
}

func (h *testSyncHandler) manager() *testMessagePool {
	if h.SyncHandler != nil {
		return h.SyncHandler.(*syncTreeHandler).syncClient.MessagePool().(*testMessagePool)
	}
	return h.syncClient.MessagePool().(*testMessagePool)
}

func (h *testSyncHandler) tree() *broadcastTree {
	return h.SyncHandler.(*syncTreeHandler).objTree.(*broadcastTree)
}

func (h *testSyncHandler) send(ctx context.Context, msg protocolMsg) (err error) {
	return h.batcher.Add(ctx, msg)
}

func (h *testSyncHandler) sendRawChanges(ctx context.Context, changes objecttree.RawChangesPayload) {
	h.batcher.Add(ctx, protocolMsg{userMsg: &changes})
}

func (h *testSyncHandler) run(ctx context.Context, t *testing.T, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			res, err := h.batcher.WaitOne(ctx)
			if err != nil {
				return
			}
			if res.userMsg != nil {
				h.tree().Lock()
				userRes, err := h.tree().AddRawChanges(ctx, *res.userMsg)
				require.NoError(t, err)
				fmt.Println("user add result", userRes.Heads)
				h.tree().Unlock()
				continue
			}
			err = h.HandleMessage(ctx, res.senderId, res.msg)
			if err != nil {
				fmt.Println("error handling message", err.Error())
				continue
			}
		}
	}()
}

// testMessagePool captures all other handlers and sends messages to them
type testMessagePool struct {
	peerId   string
	handlers map[string]*testSyncHandler
	log      *messageLog
}

func newTestMessagePool(peerId string, log *messageLog) *testMessagePool {
	return &testMessagePool{handlers: map[string]*testSyncHandler{}, peerId: peerId, log: log}
}

func (m *testMessagePool) addHandler(peerId string, handler *testSyncHandler) {
	m.handlers[peerId] = handler
}

func (m *testMessagePool) SendPeer(ctx context.Context, peerId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	pMsg := protocolMsg{
		msg:        msg,
		senderId:   m.peerId,
		receiverId: peerId,
	}
	m.log.addMessage(pMsg)
	return m.handlers[peerId].send(context.Background(), pMsg)
}

func (m *testMessagePool) Broadcast(ctx context.Context, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	for _, handler := range m.handlers {
		pMsg := protocolMsg{
			msg:        msg,
			senderId:   m.peerId,
			receiverId: handler.peerId,
		}
		m.log.addMessage(pMsg)
		handler.send(context.Background(), pMsg)
	}
	return
}

func (m *testMessagePool) GetResponsiblePeers(ctx context.Context) (peers []peer.Peer, err error) {
	panic("should not be called")
}

func (m *testMessagePool) LastUsage() time.Time {
	panic("should not be called")
}

func (m *testMessagePool) HandleMessage(ctx context.Context, senderId string, request *spacesyncproto.ObjectSyncMessage) (err error) {
	panic("should not be called")
}

func (m *testMessagePool) SendSync(ctx context.Context, peerId string, message *spacesyncproto.ObjectSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error) {
	panic("should not be called")
}

// broadcastTree is the tree that broadcasts changes to everyone when changes are added
// it is a simplified version of SyncTree which is easier to use in the test environment
type broadcastTree struct {
	objecttree.ObjectTree
	objectsync.SyncClient
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

func createStorage(treeId string, aclList list.AclList) treestorage.TreeStorage {
	changeCreator := objecttree.NewMockChangeCreator()
	st := changeCreator.CreateNewTreeStorage(treeId, aclList.Head().Id)
	return st
}

func createTestTree(aclList list.AclList, storage treestorage.TreeStorage) (objecttree.ObjectTree, error) {
	return objecttree.BuildEmptyDataTestableTree(storage, aclList)
}

type fixtureDeps struct {
	aclList       list.AclList
	initStorage   *treestorage.InMemoryTreeStorage
	connectionMap map[string][]string
	emptyTrees    []string
}

// protocolFixture is the test environment for sync protocol tests
type protocolFixture struct {
	handlers map[string]*testSyncHandler
	log      *messageLog
	wg       *sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

func newProtocolFixture(t *testing.T, spaceId string, deps fixtureDeps) *protocolFixture {
	var (
		handlers    = map[string]*testSyncHandler{}
		log         = newMessageLog()
		wg          = sync.WaitGroup{}
		ctx, cancel = context.WithCancel(context.Background())
	)

	for peerId := range deps.connectionMap {
		var handler *testSyncHandler
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
	return &protocolFixture{
		handlers: handlers,
		log:      log,
		wg:       &wg,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (p *protocolFixture) run(t *testing.T) {
	for _, handler := range p.handlers {
		handler.run(p.ctx, t, p.wg)
	}
}

func (p *protocolFixture) stop() {
	p.cancel()
	p.wg.Wait()
}

// genParams is the parameters for genChanges
type genParams struct {
	// prefix is the prefix which is added to change id
	prefix     string
	aclId      string
	startIdx   int
	levels     int
	perLevel   int
	snapshotId string
	prevHeads  []string
	isSnapshot func() bool
}

// genResult is the result of genChanges
type genResult struct {
	changes    []*treechangeproto.RawTreeChangeWithId
	heads      []string
	snapshotId string
}

// genChanges generates several levels of tree changes where each level is connected only with previous one
func genChanges(creator *objecttree.MockChangeCreator, params genParams) (res genResult) {
	src := rand.NewSource(time.Now().Unix())
	rnd := rand.New(src)
	var (
		prevHeads  []string
		snapshotId = params.snapshotId
	)
	prevHeads = append(prevHeads, params.prevHeads...)

	for i := 0; i < params.levels; i++ {
		var (
			newHeads []string
			usedIds  = map[string]struct{}{}
		)
		newChange := func(isSnapshot bool, idx int, prevIds []string) string {
			newId := fmt.Sprintf("%s.%d.%d", params.prefix, params.startIdx+i, idx)
			newCh := creator.CreateRaw(newId, params.aclId, snapshotId, isSnapshot, prevIds...)
			res.changes = append(res.changes, newCh)
			return newId
		}
		if params.isSnapshot() {
			newId := newChange(true, 0, prevHeads)
			prevHeads = []string{newId}
			snapshotId = newId
			continue
		}
		perLevel := rnd.Intn(params.perLevel)
		if perLevel == 0 {
			perLevel = 1
		}
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
			var prevIds []string
			for k := 0; k < prevConns; k++ {
				prevIds = append(prevIds, prevHeads[k])
				usedIds[prevHeads[k]] = struct{}{}
			}
			newId := newChange(false, j, prevIds)
			newHeads = append(newHeads, newId)
		}
		prevHeads = newHeads
	}
	res.heads = prevHeads
	res.snapshotId = snapshotId
	return
}

package synctree

import (
	"context"
	"fmt"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/objectsync/synchandler"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/net/peer"
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

type requestPeerManager struct {
	peerId   string
	handlers map[string]*testSyncHandler
	log      *messageLog
}

func newRequestPeerManager(peerId string, log *messageLog) *requestPeerManager {
	return &requestPeerManager{
		peerId:   peerId,
		handlers: map[string]*testSyncHandler{},
		log:      log,
	}
}

func (r *requestPeerManager) addHandler(peerId string, handler *testSyncHandler) {
	r.handlers[peerId] = handler
}

func (r *requestPeerManager) Run(ctx context.Context) (err error) {
	return nil
}

func (r *requestPeerManager) Close(ctx context.Context) (err error) {
	return nil
}

func (r *requestPeerManager) SendRequest(ctx context.Context, peerId string, msg *spacesyncproto.ObjectSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error) {
	panic("should not be called")
}

func (r *requestPeerManager) QueueRequest(peerId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	pMsg := protocolMsg{
		msg:        msg,
		senderId:   r.peerId,
		receiverId: peerId,
	}
	r.log.addMessage(pMsg)
	return r.handlers[peerId].send(context.Background(), pMsg)
}

func (r *requestPeerManager) Init(a *app.App) (err error) {
	return
}

func (r *requestPeerManager) Name() (name string) {
	return
}

func (r *requestPeerManager) SendPeer(ctx context.Context, peerId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	pMsg := protocolMsg{
		msg:        msg,
		senderId:   r.peerId,
		receiverId: peerId,
	}
	r.log.addMessage(pMsg)
	return r.handlers[peerId].send(context.Background(), pMsg)
}

func (r *requestPeerManager) Broadcast(ctx context.Context, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	for _, handler := range r.handlers {
		pMsg := protocolMsg{
			msg:        msg,
			senderId:   r.peerId,
			receiverId: handler.peerId,
		}
		r.log.addMessage(pMsg)
		handler.send(context.Background(), pMsg)
	}
	return
}

func (r *requestPeerManager) GetResponsiblePeers(ctx context.Context) (peers []peer.Peer, err error) {
	return nil, nil
}

func (r *requestPeerManager) GetNodePeers(ctx context.Context) (peers []peer.Peer, err error) {
	return nil, nil
}

// testSyncHandler is the wrapper around individual tree to test sync protocol
type testSyncHandler struct {
	synchandler.SyncHandler
	batcher     *mb.MB[protocolMsg]
	peerId      string
	aclList     list.AclList
	log         *messageLog
	syncClient  SyncClient
	builder     objecttree.BuildObjectTreeFunc
	peerManager *requestPeerManager
	counter     *operationCounter
}

// createSyncHandler creates a sync handler when a tree is already created
func createSyncHandler(peerId, spaceId string, objTree objecttree.ObjectTree, log *messageLog) *testSyncHandler {
	peerManager := newRequestPeerManager(peerId, log)
	syncClient := NewSyncClient(spaceId, peerManager, peerManager)
	netTree := &broadcastTree{
		ObjectTree: objTree,
		SyncClient: syncClient,
	}
	handler := newSyncTreeHandler(spaceId, netTree, syncClient, syncstatus.NewNoOpSyncStatus())
	return &testSyncHandler{
		SyncHandler: handler,
		batcher:     mb.New[protocolMsg](0),
		peerId:      peerId,
		peerManager: peerManager,
	}
}

// createEmptySyncHandler creates a sync handler when the tree will be provided later (this emulates the situation when we have no tree)
func createEmptySyncHandler(peerId, spaceId string, builder objecttree.BuildObjectTreeFunc, aclList list.AclList, log *messageLog) *testSyncHandler {
	peerManager := newRequestPeerManager(peerId, log)
	syncClient := NewSyncClient(spaceId, peerManager, peerManager)

	batcher := mb.New[protocolMsg](0)
	return &testSyncHandler{
		batcher:     batcher,
		peerId:      peerId,
		aclList:     aclList,
		log:         log,
		syncClient:  syncClient,
		builder:     builder,
		peerManager: peerManager,
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
		newTreeRequest := NewRequestFactory().CreateNewTreeRequest()
		return h.syncClient.QueueRequest(senderId, request.ObjectId, newTreeRequest)
	}
	fullSyncResponse := unmarshalled.Content.GetFullSyncResponse()
	treeStorage, _ := treestorage.NewInMemoryTreeStorage(unmarshalled.RootChange, []string{unmarshalled.RootChange.Id}, nil)
	tree, err := h.builder(treeStorage, h.aclList)
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
	headUpdate := NewRequestFactory().CreateHeadUpdate(netTree, res.Added)
	h.syncClient.Broadcast(headUpdate)
	return nil
}

func (h *testSyncHandler) manager() *requestPeerManager {
	return h.peerManager
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
				h.counter.Increment()
				h.tree().Lock()
				userRes, err := h.tree().AddRawChanges(ctx, *res.userMsg)
				require.NoError(t, err)
				fmt.Println("user add result", userRes.Heads)
				h.tree().Unlock()
				h.counter.Decrement()
				continue
			}
			if res.description().name == "FullSyncRequest" {
				h.counter.Increment()
				resp, err := h.HandleRequest(ctx, res.senderId, res.msg)
				h.counter.Decrement()
				if err != nil {
					fmt.Println("error handling request", err.Error())
					continue
				}
				h.peerManager.SendPeer(ctx, res.senderId, resp)
			} else {
				h.counter.Increment()
				err = h.HandleMessage(ctx, res.senderId, res.msg)
				h.counter.Decrement()
				if err != nil {
					fmt.Println("error handling message", err.Error())
				}
			}
		}
	}()
}

// broadcastTree is the tree that broadcasts changes to everyone when changes are added
// it is a simplified version of SyncTree which is easier to use in the test environment
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
	b.SyncClient.Broadcast(upd)
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
	counter       *operationCounter
	aclList       list.AclList
	initStorage   *treestorage.InMemoryTreeStorage
	connectionMap map[string][]string
	emptyTrees    []string
	treeBuilder   objecttree.BuildObjectTreeFunc
}

// protocolFixture is the test environment for sync protocol tests
type protocolFixture struct {
	handlers map[string]*testSyncHandler
	log      *messageLog
	wg       *sync.WaitGroup
	counter  *operationCounter
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
			handler = createEmptySyncHandler(peerId, spaceId, deps.treeBuilder, deps.aclList, log)
		} else {
			stCopy := deps.initStorage.Copy()
			testTree, err := deps.treeBuilder(stCopy, deps.aclList)
			require.NoError(t, err)
			handler = createSyncHandler(peerId, spaceId, testTree, log)
		}
		handler.counter = deps.counter
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
		counter:  deps.counter,
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
	hasData    bool
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
			var data []byte
			if params.hasData {
				data = []byte(newId)
			}
			newCh := creator.CreateRawWithData(newId, params.aclId, snapshotId, isSnapshot, data, prevIds...)
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

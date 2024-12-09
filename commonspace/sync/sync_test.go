package sync

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/anyproto/protobuf/proto"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/peermanager"
	"github.com/anyproto/any-sync/commonspace/peermanager/mock_peermanager"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/mock_nodeconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/any-sync/testutil/anymock"
	"github.com/anyproto/any-sync/util/syncqueues"
)

var ctx = context.Background()

func TestSyncService(t *testing.T) {
	t.Run("send and receive", func(t *testing.T) {
		f := newFixture(t)
		f.syncHandler.toSendData = map[string][]*testResponse{
			"objectId": {
				{msg: "send-msg1"},
				{msg: "send-msg2"},
			},
		}
		f.syncHandler.toReceiveData = map[string][]*testResponse{
			"objectId": {
				{msg: "receive-msg1"},
				{msg: "receive-msg2"},
			},
		}
		f.syncHandler.responsesExist = true
		rq := &testRequest{peerId: "peerId", objectId: "objectId"}
		returnRq := &testRequest{peerId: "peerId1", objectId: "objectId"}
		handleStream := f.syncHandler.newSendStream(ctx, "objectId")
		f.syncHandler.returnRequest = returnRq
		err := f.HandleStreamRequest(ctx, rq, handleStream)
		require.NoError(t, err)
		f.Close(t)
		for i, resp := range handleStream.(*testStream).toReceive {
			require.Equal(t, f.syncHandler.toSendData["objectId"][i], resp)
		}
		for i, resp := range f.syncHandler.toReceiveData["objectId"] {
			require.Equal(t, f.syncHandler.collector.responses[i].resp, resp)
			require.Equal(t, f.syncHandler.collector.responses[i].peerId, returnRq.peerId)
		}
	})
	t.Run("send", func(t *testing.T) {
		f := newFixture(t)
		defer f.Close(t)
		f.syncHandler.toReceiveData = map[string][]*testResponse{
			"objectId": {
				{msg: "receive-msg1"},
				{msg: "receive-msg2"},
			},
		}
		rq := &testRequest{peerId: "peerId", objectId: "objectId"}
		err := f.SendRequest(ctx, rq, f.syncHandler.collector)
		require.NoError(t, err)
		for i, resp := range f.syncHandler.toReceiveData["objectId"] {
			require.Equal(t, f.syncHandler.collector.responses[i].resp, resp)
			require.Equal(t, f.syncHandler.collector.responses[i].peerId, rq.peerId)
		}
	})
	t.Run("queue", func(t *testing.T) {
		f := newFixture(t)
		f.syncHandler.toReceiveData = map[string][]*testResponse{
			"objectId": {
				{msg: "receive-msg1"},
				{msg: "receive-msg2"},
			},
		}
		f.syncHandler.responsesExist = true
		rq := &testRequest{peerId: "peerId", objectId: "objectId"}
		err := f.QueueRequest(ctx, rq)
		require.NoError(t, err)
		f.Close(t)
		for i, resp := range f.syncHandler.toReceiveData["objectId"] {
			require.Equal(t, f.syncHandler.collector.responses[i].resp, resp)
			require.Equal(t, f.syncHandler.collector.responses[i].peerId, rq.peerId)
		}
	})
	t.Run("handle message and queue", func(t *testing.T) {
		f := newFixture(t)
		f.syncHandler.toReceiveData = map[string][]*testResponse{
			"objectId": {
				{msg: "receive-msg1"},
				{msg: "receive-msg2"},
			},
		}
		f.syncHandler.responsesExist = true
		f.syncHandler.headUpdateExist = true
		headUpdate := &testMessage{objectId: "objectId"}
		rq := &testRequest{peerId: "peerId", objectId: "objectId"}
		f.syncHandler.returnRequest = rq
		err := f.HandleMessage(ctx, headUpdate)
		require.NoError(t, err)
		f.Close(t)
		for i, resp := range f.syncHandler.toReceiveData["objectId"] {
			require.Equal(t, f.syncHandler.collector.responses[i].resp, resp)
			require.Equal(t, f.syncHandler.collector.responses[i].peerId, rq.peerId)
		}
		require.Equal(t, headUpdate, f.syncHandler.headUpdate)
	})
	t.Run("handle message", func(t *testing.T) {
		f := newFixture(t)
		f.syncHandler.toReceiveData = map[string][]*testResponse{
			"objectId": {
				{msg: "receive-msg1"},
				{msg: "receive-msg2"},
			},
		}
		f.syncHandler.headUpdateExist = true
		headUpdate := &testMessage{objectId: "objectId"}
		err := f.HandleMessage(ctx, headUpdate)
		require.NoError(t, err)
		f.Close(t)
		require.Equal(t, headUpdate, f.syncHandler.headUpdate)
		require.Empty(t, f.syncHandler.collector.responses)
	})
	t.Run("send EOF", func(t *testing.T) {
		f := newFixture(t)
		defer f.Close(t)
		f.syncHandler.toReceiveData = map[string][]*testResponse{
			"objectId": {},
		}
		rq := &testRequest{peerId: "peerId", objectId: "objectId"}
		err := f.SendRequest(ctx, rq, f.syncHandler.collector)
		require.Equal(t, io.EOF, err)
	})
	t.Run("ignore handling same requests", func(t *testing.T) {
		f := newFixture(t)
		defer f.Close(t)
		f.syncService.manager.(*requestManager).incomingGuard.TryTake(fullId("peerId", "objectId"))
		rq := &testRequest{peerId: "peerId", objectId: "objectId"}
		handleStream := f.syncHandler.newSendStream(ctx, "objectId")
		err := f.HandleStreamRequest(ctx, rq, handleStream)
		require.Equal(t, spacesyncproto.ErrDuplicateRequest, err)
	})
}

type fixture struct {
	*syncService
	a           *app.App
	nodeConf    *mock_nodeconf.MockService
	syncHandler *testSyncHandler
	spaceState  *spacestate.SpaceState
	peerManager *mock_peermanager.MockPeerManager
	syncQueues  syncqueues.SyncQueues
	ctrl        *gomock.Controller
}

func (fx *fixture) Close(t *testing.T) {
	require.NoError(t, fx.a.Close(context.Background()))
	fx.ctrl.Finish()
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.a = &app.App{}
	f.ctrl = gomock.NewController(t)
	f.nodeConf = mock_nodeconf.NewMockService(f.ctrl)
	f.syncHandler = &testSyncHandler{
		toReceiveData: map[string][]*testResponse{},
		toSendData:    map[string][]*testResponse{},
		collector:     &testResponseCollector{},
	}
	accService := &accounttest.AccountTestService{}
	f.spaceState = &spacestate.SpaceState{SpaceId: "spaceId"}
	f.peerManager = mock_peermanager.NewMockPeerManager(f.ctrl)
	anymock.ExpectComp(f.peerManager.EXPECT(), peermanager.CName)
	anymock.ExpectComp(f.nodeConf.EXPECT(), nodeconf.CName)
	f.nodeConf.EXPECT().Configuration().Return(nodeconf.Configuration{})
	f.syncQueues = syncqueues.New()
	f.syncService = &syncService{}
	f.a.Register(accService).
		Register(f.peerManager).
		Register(f.spaceState).
		Register(f.nodeConf).
		Register(f.syncQueues).
		Register(f.syncService).
		Register(f.syncHandler)

	require.NoError(t, f.a.Start(context.Background()))
	return f
}

type mockEncoding struct {
}

func (m mockEncoding) Marshal(msg drpc.Message) ([]byte, error) {
	return nil, nil
}

func (m mockEncoding) Unmarshal(buf []byte, msg drpc.Message) error {
	return nil
}

type testSyncHandler struct {
	toSendData      map[string][]*testResponse
	toReceiveData   map[string][]*testResponse
	responsesExist  bool
	headUpdateExist bool
	collector       *testResponseCollector
	returnRequest   syncdeps.Request
	headUpdate      *testMessage
}

func (m *testSyncHandler) Run(ctx context.Context) (err error) {
	return nil
}

func (m *testSyncHandler) Close(ctx context.Context) (err error) {
	var (
		responsesFound  = !m.responsesExist
		headUpdateFound = !m.headUpdateExist
	)
	for {
		if responsesFound && headUpdateFound {
			break
		}
		m.collector.Lock()
		if !responsesFound && len(m.collector.responses) != 0 {
			responsesFound = true
		}
		if !headUpdateFound && m.headUpdate != nil {
			headUpdateFound = true
		}
		m.collector.Unlock()
		if responsesFound && headUpdateFound {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func (m *testSyncHandler) newSendStream(ctx context.Context, objectId string) drpc.Stream {
	return &testStream{ctx: ctx}
}

func (m *testSyncHandler) newReceiveStream(ctx context.Context, objectId string) drpc.Stream {
	return &testStream{ctx: ctx, toSend: m.toReceiveData[objectId]}
}

func (m *testSyncHandler) Init(a *app.App) (err error) {
	return
}

func (m *testSyncHandler) Name() (name string) {
	return syncdeps.CName
}

func (m *testSyncHandler) HandleHeadUpdate(ctx context.Context, headUpdate drpc.Message) (syncdeps.Request, error) {
	m.collector.Lock()
	defer m.collector.Unlock()
	m.headUpdate = headUpdate.(*testMessage)
	return m.returnRequest, nil
}

func (m *testSyncHandler) HandleStreamRequest(ctx context.Context, rq syncdeps.Request, updater syncdeps.QueueSizeUpdater, sendResponse func(resp proto.Message) error) (syncdeps.Request, error) {
	for _, resp := range m.toSendData[rq.ObjectId()] {
		if err := sendResponse(resp); err != nil {
			return nil, err
		}
	}
	return m.returnRequest, nil
}

func (m *testSyncHandler) HandleDeprecatedObjectSync(ctx context.Context, req *spacesyncproto.ObjectSyncMessage) (resp *spacesyncproto.ObjectSyncMessage, err error) {
	return nil, nil
}

func (m *testSyncHandler) ApplyRequest(ctx context.Context, rq syncdeps.Request, requestSender syncdeps.RequestSender) error {
	return requestSender.SendRequest(ctx, rq, m.collector)
}

func (m *testSyncHandler) SendStreamRequest(ctx context.Context, rq syncdeps.Request, receive func(stream drpc.Stream) error) (err error) {
	return receive(m.newReceiveStream(ctx, rq.ObjectId()))
}

type testStream struct {
	ctx       context.Context
	toSend    []*testResponse
	toReceive []*testResponse
}

func (t *testStream) Context() context.Context {
	return t.ctx
}

func (t *testStream) MsgSend(msg drpc.Message, enc drpc.Encoding) error {
	t.toReceive = append(t.toReceive, msg.(*testResponse))
	return nil
}

func (t *testStream) MsgRecv(msg drpc.Message, enc drpc.Encoding) error {
	if len(t.toSend) == 0 {
		return io.EOF
	}
	msg.(*testResponse).msg = t.toSend[0].msg
	t.toSend = t.toSend[1:]
	return nil
}

func (t *testStream) CloseSend() error {
	return nil
}

func (t *testStream) Close() error {
	return nil
}

type collectedResponse struct {
	peerId   string
	objectId string
	resp     syncdeps.Response
}

type testResponseCollector struct {
	responses []collectedResponse
	sync.Mutex
}

func (t *testResponseCollector) NewResponse() syncdeps.Response {
	return &testResponse{}
}

func (t *testResponseCollector) CollectResponse(ctx context.Context, peerId, objectId string, resp syncdeps.Response) error {
	t.Lock()
	defer t.Unlock()
	t.responses = append(t.responses, collectedResponse{peerId: peerId, objectId: objectId, resp: resp})
	return nil
}

type testResponse struct {
	msg string
}

func (t *testResponse) Reset() {
}

func (t *testResponse) String() string {
	return t.msg
}

func (t *testResponse) ProtoMessage() {
}

func (t *testResponse) MsgSize() uint64 {
	return uint64(len(t.msg))
}

type testRequest struct {
	peerId   string
	objectId string
}

func (r *testRequest) PeerId() string {
	return r.peerId
}

func (r *testRequest) ObjectId() string {
	return r.objectId
}

func (r *testRequest) Proto() (proto.Message, error) {
	return nil, nil
}

func (r *testRequest) MsgSize() uint64 {
	return 0
}

type testMessage struct {
	objectId string
}

func (t *testMessage) ObjectId() string {
	return t.objectId
}

func (t *testMessage) MsgSize() uint64 {
	return uint64(len(t.objectId))
}

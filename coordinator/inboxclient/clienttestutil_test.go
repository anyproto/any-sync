package inboxclient

import (
	"context"
	"testing"
	"time"

	accountService "github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
	"github.com/anyproto/any-sync/coordinator/inboxclient/mocks"
	"github.com/anyproto/any-sync/coordinator/subscribeclient"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/go-chash"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var coordinatorPeer = "peer"

func newFixtureServer(t *testing.T) (fx *fixtureServer) {
	testServer := &testServer{
		NotifySenderChan: make(chan *coordinatorproto.NotifySubscribeEvent),
	}

	ts := rpctest.NewTestServer()
	account := &accounttest.AccountTestService{}

	fx = &fixtureServer{
		account:    account,
		a:          new(app.App),
		testServer: testServer,
		ts:         ts,
		tp:         rpctest.NewTestPool().WithServer(ts),
	}

	fx.a.
		Register(account).
		Register(&mockConf{}).
		Register(fx.tp).
		Register(fx.ts)

	require.NoError(t, coordinatorproto.DRPCRegisterCoordinator(ts, testServer))
	require.NoError(t, fx.a.Start(ctx))

	return fx
}

func newFixtureClient(t *testing.T) (fx *fixtureClient) {
	ts := rpctest.NewTestServer()
	account := &accounttest.AccountTestService{}
	ctrl := gomock.NewController(t)
	mockReceiver := mocks.NewMockMessageReceiverTest(ctrl)
	inboxClient := New()

	fx = &fixtureClient{
		account:      account,
		inbox:        inboxClient,
		subscribe:    subscribeclient.New(),
		ctrl:         ctrl,
		a:            new(app.App),
		ts:           ts,
		tp:           rpctest.NewTestPool().WithServer(ts),
		mockReceiver: mockReceiver,
	}
	fx.inbox.SetMessageReceiver(mockReceiver.Receive)
	fx.a.
		Register(account).
		Register(&mockConf{}).
		Register(fx.tp).
		Register(fx.ts).
		Register(fx.subscribe).
		Register(fx.inbox)

	require.NoError(t, fx.a.Start(ctx))

	return fx
}

type fixtureServer struct {
	account    *accounttest.AccountTestService
	a          *app.App
	testServer *testServer
	ts         *rpctest.TestServer
	tp         *rpctest.TestPool
}

type fixtureClient struct {
	inbox        InboxClient
	account      *accounttest.AccountTestService
	subscribe    subscribeclient.SubscribeClientService
	a            *app.App
	ctrl         *gomock.Controller
	ts           *rpctest.TestServer
	tp           *rpctest.TestPool
	mockReceiver *mocks.MockMessageReceiverTest
}

func (fx *fixtureServer) Finish(t *testing.T) {
	require.NoError(t, fx.a.Close(ctx))
}

func (fx *fixtureClient) Finish(t *testing.T) {
	require.NoError(t, fx.a.Close(ctx))
	fx.ctrl.Finish()

}

type testServer struct {
	coordinatorproto.DRPCCoordinatorUnimplementedServer
	FetchResponse    *coordinatorproto.InboxFetchResponse
	NotifySenderChan chan *coordinatorproto.NotifySubscribeEvent
}

func (t *testServer) InboxAddMessage(ctx context.Context, in *coordinatorproto.InboxAddMessageRequest) (*coordinatorproto.InboxAddMessageResponse, error) {
	t.FetchResponse.Messages = append(t.FetchResponse.Messages, in.Message)
	e := &coordinatorproto.NotifySubscribeEvent{
		EventType: coordinatorproto.NotifyEventType_InboxNewMessageEvent,
		Payload:   []byte(in.Message.Id),
	}
	t.NotifySenderChan <- e
	return &coordinatorproto.InboxAddMessageResponse{}, nil
}

func (t *testServer) InboxFetch(context.Context, *coordinatorproto.InboxFetchRequest) (*coordinatorproto.InboxFetchResponse, error) {
	return t.FetchResponse, nil
}

func (t *testServer) NotifySubscribe(req *coordinatorproto.NotifySubscribeRequest, rpcStream coordinatorproto.DRPCCoordinator_NotifySubscribeStream) error {
	closeCh := make(chan struct{})
	go t.notifySender(rpcStream, closeCh)
	<-rpcStream.Context().Done()
	close(closeCh)
	return nil
}

func (t *testServer) notifySender(rpcStream coordinatorproto.DRPCCoordinator_NotifySubscribeStream, closeCh chan struct{}) {
	select {
	case e := <-t.NotifySenderChan:
		rpcStream.Send(e)
	case <-closeCh:
		return
	}
}

// // //// // //// // //
type mockConf struct {
	id            string
	networkId     string
	configuration nodeconf.Configuration
}

func (m *mockConf) NetworkCompatibilityStatus() nodeconf.NetworkCompatibilityStatus {
	return nodeconf.NetworkCompatibilityStatusOk
}

func (m *mockConf) Init(a *app.App) (err error) {
	accountKeys := a.MustComponent(accountService.CName).(accountService.Service).Account()

	sk := accountKeys.SignKey
	networkId := sk.GetPublic().Network()
	node := nodeconf.Node{
		PeerId:    accountKeys.PeerId,
		Addresses: []string{"127.0.0.1:4430"},
		Types:     []nodeconf.NodeType{nodeconf.NodeTypeTree},
	}
	m.id = networkId
	m.networkId = networkId
	m.configuration = nodeconf.Configuration{
		Id:           networkId,
		NetworkId:    networkId,
		Nodes:        []nodeconf.Node{node},
		CreationTime: time.Now(),
	}
	return nil
}

func (m *mockConf) Name() (name string) {
	return nodeconf.CName
}

func (m *mockConf) Run(ctx context.Context) (err error) {
	return nil
}

func (m *mockConf) Close(ctx context.Context) (err error) {
	return nil
}

func (m *mockConf) Id() string {
	return ""
}

func (m *mockConf) Configuration() nodeconf.Configuration {
	return m.configuration
}

func (m *mockConf) NodeIds(spaceId string) []string {
	var nodeIds []string
	return nodeIds
}

func (m *mockConf) IsResponsible(spaceId string) bool {
	return true
}

func (m *mockConf) FilePeers() []string {
	return nil
}

func (m *mockConf) ConsensusPeers() []string {
	return nil
}

func (m *mockConf) CoordinatorPeers() []string {
	return []string{coordinatorPeer}
}

func (m *mockConf) NamingNodePeers() []string {
	return nil
}

func (m *mockConf) PaymentProcessingNodePeers() []string {
	return nil
}

func (m *mockConf) PeerAddresses(peerId string) (addrs []string, ok bool) {
	return nil, false
}

func (m *mockConf) CHash() chash.CHash {
	return nil
}

func (m *mockConf) Partition(spaceId string) (part int) {
	return 0
}

func (m *mockConf) NodeTypes(nodeId string) []nodeconf.NodeType {
	return nil
}

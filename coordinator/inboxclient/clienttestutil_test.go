package inboxclient

import (
	"context"
	"testing"
	"time"

	accountService "github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
	"github.com/anyproto/any-sync/coordinator/inboxclient/mocks"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/go-chash"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var coordinatorPeer = "peer"

// myTs is used to send testServer structure to control e.g. what
// InboxFetch returns
func newFixtureWithReceiver(t *testing.T, myTs *testServer) (fx *fixture) {
	ts := rpctest.NewTestServer()
	account := &accounttest.AccountTestService{}
	c := New()
	ctrl := gomock.NewController(t)
	mockReceiver := mocks.NewMockMessageReceiverTest(ctrl)

	fx = &fixture{
		InboxClient:  c,
		account:      account,
		ctrl:         ctrl,
		a:            new(app.App),
		ts:           ts,
		tp:           rpctest.NewTestPool().WithServer(ts),
		mockReceiver: mockReceiver,
	}

	fx.SetMessageReceiver(mockReceiver.Receive)
	fx.a.
		Register(account).
		Register(&mockConf{}).
		Register(fx.tp).
		Register(fx.ts).
		Register(c)

	if myTs == nil {
		myTs = &testServer{}
	}
	require.NoError(t, coordinatorproto.DRPCRegisterCoordinator(ts, myTs))
	require.NoError(t, fx.a.Start(ctx))

	return fx
}

func dummyReceiver(e *coordinatorproto.InboxNotifySubscribeEvent) {

}

func newFixture(t *testing.T, myTs *testServer) (fx *fixture) {
	ts := rpctest.NewTestServer()
	account := &accounttest.AccountTestService{}
	c := New()
	ctrl := gomock.NewController(t)

	fx = &fixture{
		InboxClient: c,
		account:     account,
		ctrl:        ctrl,
		a:           new(app.App),
		ts:          ts,
		tp:          rpctest.NewTestPool().WithServer(ts),
	}

	fx.SetMessageReceiver(dummyReceiver)
	fx.a.
		Register(account).
		Register(&mockConf{}).
		Register(fx.tp).
		Register(fx.ts).
		Register(c)

	if myTs == nil {
		myTs = &testServer{}
	}
	require.NoError(t, coordinatorproto.DRPCRegisterCoordinator(ts, myTs))
	require.NoError(t, fx.a.Start(ctx))

	return fx
}

type fixture struct {
	InboxClient
	account      *accounttest.AccountTestService
	a            *app.App
	ctrl         *gomock.Controller
	ts           *rpctest.TestServer
	tp           *rpctest.TestPool
	mockReceiver *mocks.MockMessageReceiverTest
}

func (fx *fixture) Finish(t *testing.T) {
	require.NoError(t, fx.a.Close(ctx))
	fx.ctrl.Finish()
}

type testServer struct {
	coordinatorproto.DRPCCoordinatorUnimplementedServer
	FetchResponse    *coordinatorproto.InboxFetchResponse
	NotifySenderChan chan *coordinatorproto.InboxNotifySubscribeEvent
	name             string
}

func (t *testServer) InboxAddMessage(ctx context.Context, in *coordinatorproto.InboxAddMessageRequest) (*coordinatorproto.InboxAddMessageResponse, error) {
	t.FetchResponse.Messages = append(t.FetchResponse.Messages, in.Message)
	e := &coordinatorproto.InboxNotifySubscribeEvent{
		NotifyId: "event",
	}
	t.NotifySenderChan <- e
	return &coordinatorproto.InboxAddMessageResponse{}, nil
}

func (t *testServer) InboxFetch(context.Context, *coordinatorproto.InboxFetchRequest) (*coordinatorproto.InboxFetchResponse, error) {
	return t.FetchResponse, nil
}

func (t *testServer) notifySender(rpcStream coordinatorproto.DRPCCoordinator_NotifySubscribeStream, closeCh chan struct{}) {
	select {
	case e := <-t.NotifySenderChan:
		event := &coordinatorproto.NotifySubscribeEvent{
			Event: &coordinatorproto.NotifySubscribeEvent_InboxEvent{
				InboxEvent: e,
			},
		}
		rpcStream.Send(event)
	case <-closeCh:
		return
	}
}

func (t *testServer) InboxNotifySubscribe(req *coordinatorproto.InboxNotifySubscribeRequest, rpcStream coordinatorproto.DRPCCoordinator_NotifySubscribeStream) error {
	closeCh := make(chan struct{})
	go t.notifySender(rpcStream, closeCh)
	<-rpcStream.Context().Done()
	close(closeCh)
	return nil
}

// //
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

package inboxclient

import (
	"context"
	"sync"
	"testing"
	"time"

	accountService "github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/anyproto/go-chash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/coordinator/inboxclient/mocks"
)

var ctx = context.Background()

func TestInbox_CryptoTest(t *testing.T) {
	privKey, pubKey, _ := crypto.GenerateRandomEd25519KeyPair()

	t.Run("test encrypt/decrypt", func(t *testing.T) {
		body := []byte("hello")
		encrypted, _ := pubKey.Encrypt(body)
		decrypted, _ := privKey.Decrypt(encrypted)
		assert.Equal(t, body, decrypted)
	})
}

func TestInbox_Fetch(t *testing.T) {
	var makeClientServer = func(t *testing.T, ts *testServer) (fxC, fxS *fixture, peerId string) {
		fxC = newFixture(t, nil)
		fxS = newFixture(t, ts)
		peerId = "peer"
		identity, err := fxC.account.Account().SignKey.GetPublic().Marshall()
		require.NoError(t, err)
		mcS, mcC := rpctest.MultiConnPairWithIdentity(peerId, peerId+"client", identity)
		pS, err := peer.NewPeer(mcS, fxC.ts)
		require.NoError(t, err)
		fxC.tp.AddPeer(ctx, pS)
		_, err = peer.NewPeer(mcC, fxS.ts)
		require.NoError(t, err)
		return
	}

	t.Run("simple InboxFetch", func(t *testing.T) {

		myTs := &testServer{}

		res := new(coordinatorproto.InboxFetchResponse)
		res.Messages = make([]*coordinatorproto.InboxMessage, 10)
		for i := range 10 {
			res.Messages[i] = &coordinatorproto.InboxMessage{}
		}
		myTs.FetchResponse = res
		fxC, _, _ := makeClientServer(t, myTs)
		msgs, err := fxC.InboxFetch(ctx, "")

		require.NoError(t, err)
		assert.Len(t, msgs, 10)

	})
}

func TestInbox_Notify(t *testing.T) {
	var makeClientServer = func(t *testing.T, ts *testServer) (fxC, fxS *fixture, peerId string) {
		fxC = newFixture(t, nil)
		fxS = newFixtureWithReceiver(t, ts)
		peerId = "peer"
		identity, err := fxC.account.Account().SignKey.GetPublic().Marshall()
		require.NoError(t, err)
		mcS, mcC := rpctest.MultiConnPairWithIdentity(peerId, peerId+"client", identity)
		pS, err := peer.NewPeer(mcS, fxC.ts)
		require.NoError(t, err)
		fxC.tp.AddPeer(ctx, pS)
		_, err = peer.NewPeer(mcC, fxS.ts)
		require.NoError(t, err)
		return
	}
	t.Run("notify simple test", func(t *testing.T) {

		myTs := &testServer{
			name:             "fxC",
			NotifySenderChan: make(chan *coordinatorproto.InboxNotifySubscribeEvent),
		}
		expectedEvent := &coordinatorproto.InboxNotifySubscribeEvent{
			NotifyId: "hello",
		}
		_, fxS, _ := makeClientServer(t, myTs)
		var wg sync.WaitGroup
		wg.Add(1)
		fxS.mockReceiver.EXPECT().
			Receive(expectedEvent).
			Do(func(evt *coordinatorproto.InboxNotifySubscribeEvent) {
				defer wg.Done()
			}).
			Times(1)

		myTs.NotifySenderChan <- expectedEvent

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			return
		case <-time.After(2 * time.Second):
			t.Fatal("Receive callback was not triggered in time")
		}
	})

}

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
	fmt.Printf("event: %s\n", e)
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

func (t *testServer) InboxFetch(context.Context, *coordinatorproto.InboxFetchRequest) (*coordinatorproto.InboxFetchResponse, error) {
	// TODO make ts.inboxfetch and call it here
	return t.FetchResponse, nil
}

func (t *testServer) notifySender(rpcStream coordinatorproto.DRPCCoordinator_InboxNotifySubscribeStream, closeCh chan struct{}) {
	fmt.Printf("myTs <%s>: notifySender\n", t.name)
	select {
	case e := <-t.NotifySenderChan:
		fmt.Printf("myTs <%s>: sending test event: %s\n", t.name, e.NotifyId)
		rpcStream.Send(e)
	case <-closeCh:
		fmt.Printf("closing notifySender\n")
		return
	}
}

func (t *testServer) InboxNotifySubscribe(req *coordinatorproto.InboxNotifySubscribeRequest, rpcStream coordinatorproto.DRPCCoordinator_InboxNotifySubscribeStream) error {
	fmt.Printf("calling notify subs\n")
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

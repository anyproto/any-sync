package inboxclient

import (
	"context"
	"fmt"
	"testing"
	"time"

	accountService "github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/anyproto/go-chash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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
	t.Run("test callback call", func(t *testing.T) {
		fx := newFixture(t)
		fx.SetMessageReceiver(dummyReceiver)
		msgs, err := fx.InboxFetch(context.TODO(), "")
		require.NoError(t, err)
		assert.Len(t, msgs, 10)
	})
}

func dummyReceiver(e *coordinatorproto.InboxNotifySubscribeEvent) {
	fmt.Printf("event: %s\n", e)
}
func newFixture(t *testing.T) (fx *fixture) {
	account := &accounttest.AccountTestService{}
	c := New()
	fx = &fixture{
		InboxClient: c,
		mr:          dummyReceiver,
		account:     account,
		ctrl:        gomock.NewController(t),
		a:           new(app.App),
	}
	c.SetMessageReceiver(dummyReceiver)
	fx.a.
		Register(fx.account).
		Register(&mockConf{}).
		Register(&mockPool{}).
		Register(c)

	require.NoError(t, fx.a.Start(ctx))

	return fx
}

type fixture struct {
	InboxClient
	a       *app.App
	mr      MessageReceiver
	account *accounttest.AccountTestService
	ctrl    *gomock.Controller
}

func (fx *fixture) Finish(t *testing.T) {
	require.NoError(t, fx.a.Close(ctx))
	fx.ctrl.Finish()
}

//

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
	return m.id
}

func (m *mockConf) Configuration() nodeconf.Configuration {
	return m.configuration
}

func (m *mockConf) NodeIds(spaceId string) []string {
	var nodeIds []string
	for _, node := range m.configuration.Nodes {
		nodeIds = append(nodeIds, node.PeerId)
	}
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
	return nil
}

func (m *mockConf) NamingNodePeers() []string {
	return nil
}

func (m *mockConf) PaymentProcessingNodePeers() []string {
	return nil
}

func (m *mockConf) PeerAddresses(peerId string) (addrs []string, ok bool) {
	if peerId == m.configuration.Nodes[0].PeerId {
		return m.configuration.Nodes[0].Addresses, true
	}
	return nil, false
}

func (m *mockConf) CHash() chash.CHash {
	return nil
}

func (m *mockConf) Partition(spaceId string) (part int) {
	return 0
}

func (m *mockConf) NodeTypes(nodeId string) []nodeconf.NodeType {
	if nodeId == m.configuration.Nodes[0].PeerId {
		return m.configuration.Nodes[0].Types
	}
	return nil
}

type mockPool struct {
}

func (m *mockPool) Run(ctx context.Context) (err error) {
	return nil
}

func (m *mockPool) Close(ctx context.Context) (err error) {
	return nil
}

func (m *mockPool) AddPeer(ctx context.Context, p peer.Peer) (err error) {
	return nil
}

func (m *mockPool) Init(a *app.App) (err error) {
	return nil
}

func (m *mockPool) Name() (name string) {
	return pool.CName
}

func (m *mockPool) Get(ctx context.Context, id string) (peer.Peer, error) {
	return nil, fmt.Errorf("no such peer")
}

func (m *mockPool) Dial(ctx context.Context, id string) (peer.Peer, error) {
	return nil, fmt.Errorf("can't dial peer")
}

func (m *mockPool) GetOneOf(ctx context.Context, peerIds []string) (peer.Peer, error) {
	return nil, fmt.Errorf("can't dial peer")
}

func (m *mockPool) DialOneOf(ctx context.Context, peerIds []string) (peer.Peer, error) {
	return nil, fmt.Errorf("can't dial peer")
}

func (m *mockPool) Pick(ctx context.Context, id string) (peer.Peer, error) {
	return nil, fmt.Errorf("no such peer")
}

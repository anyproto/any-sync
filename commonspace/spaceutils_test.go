package commonspace

import (
	"context"
	"fmt"
	accountService "github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/ocache"
	"github.com/anyproto/any-sync/commonspace/config"
	"github.com/anyproto/any-sync/commonspace/credentialprovider"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/objecttreebuilder"
	"github.com/anyproto/any-sync/commonspace/peermanager"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/go-chash"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

//
// Mock NodeConf implementation
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
	networkId := accountKeys.SignKey.GetPublic().Network()
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

//
// Mock PeerManager
//

type mockPeerManager struct {
}

func (p *mockPeerManager) Init(a *app.App) (err error) {
	return nil
}

func (p *mockPeerManager) Name() (name string) {
	return peermanager.CName
}

func (p *mockPeerManager) SendPeer(ctx context.Context, peerId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	return nil
}

func (p *mockPeerManager) Broadcast(ctx context.Context, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	return nil
}

func (p *mockPeerManager) GetResponsiblePeers(ctx context.Context) (peers []peer.Peer, err error) {
	return nil, nil
}

//
// Mock PeerManagerProvider
//

type mockPeerManagerProvider struct {
}

func (m *mockPeerManagerProvider) Init(a *app.App) (err error) {
	return nil
}

func (m *mockPeerManagerProvider) Name() (name string) {
	return peermanager.CName
}

func (m *mockPeerManagerProvider) NewPeerManager(ctx context.Context, spaceId string) (sm peermanager.PeerManager, err error) {
	return &mockPeerManager{}, nil
}

//
// Mock Pool
//

type mockPool struct {
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

//
// Mock Config
//

type mockConfig struct {
}

func (m *mockConfig) Init(a *app.App) (err error) {
	return nil
}

func (m *mockConfig) Name() (name string) {
	return "config"
}

func (m *mockConfig) GetSpace() config.Config {
	return config.Config{
		GCTTL:                60,
		SyncPeriod:           20,
		KeepTreeDataInMemory: true,
	}
}

//
// Mock TreeManager
//

type noOpSyncer struct {
}

func (n noOpSyncer) Init() {
}

func (n noOpSyncer) SyncAll(ctx context.Context, peerId string, existing, missing []string) error {
	return nil
}

func (n noOpSyncer) Close() error {
	return nil
}

type mockTreeManager struct {
	space      Space
	cache      ocache.OCache
	deletedIds []string
	markedIds  []string
}

func (t *mockTreeManager) NewTreeSyncer(spaceId string, treeManager treemanager.TreeManager) treemanager.TreeSyncer {
	return noOpSyncer{}
}

func (t *mockTreeManager) MarkTreeDeleted(ctx context.Context, spaceId, treeId string) error {
	t.markedIds = append(t.markedIds, treeId)
	return nil
}

func (t *mockTreeManager) Init(a *app.App) (err error) {
	t.cache = ocache.New(func(ctx context.Context, id string) (value ocache.Object, err error) {
		return t.space.TreeBuilder().BuildTree(ctx, id, objecttreebuilder.BuildTreeOpts{})
	},
		ocache.WithGCPeriod(time.Minute),
		ocache.WithTTL(time.Duration(60)*time.Second))
	return nil
}

func (t *mockTreeManager) Name() (name string) {
	return treemanager.CName
}

func (t *mockTreeManager) Run(ctx context.Context) (err error) {
	return nil
}

func (t *mockTreeManager) Close(ctx context.Context) (err error) {
	return t.cache.Close()
}

func (t *mockTreeManager) GetTree(ctx context.Context, spaceId, treeId string) (objecttree.ObjectTree, error) {
	val, err := t.cache.Get(ctx, treeId)
	if err != nil {
		return nil, err
	}
	return val.(objecttree.ObjectTree), nil
}

func (t *mockTreeManager) DeleteTree(ctx context.Context, spaceId, treeId string) (err error) {
	tr, err := t.GetTree(ctx, spaceId, treeId)
	if err != nil {
		return
	}
	err = tr.Delete()
	if err != nil {
		return
	}
	t.deletedIds = append(t.deletedIds, treeId)
	_, err = t.cache.Remove(ctx, treeId)
	return nil
}

//
// Space fixture
//

type spaceFixture struct {
	app                  *app.App
	config               *mockConfig
	account              accountService.Service
	configurationService nodeconf.Service
	storageProvider      spacestorage.SpaceStorageProvider
	peermanagerProvider  peermanager.PeerManagerProvider
	credentialProvider   credentialprovider.CredentialProvider
	treeManager          *mockTreeManager
	pool                 *mockPool
	spaceService         SpaceService
	cancelFunc           context.CancelFunc
}

func newFixture(t *testing.T) *spaceFixture {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	fx := &spaceFixture{
		cancelFunc:           cancel,
		config:               &mockConfig{},
		app:                  &app.App{},
		account:              &accounttest.AccountTestService{},
		configurationService: &mockConf{},
		storageProvider:      spacestorage.NewInMemorySpaceStorageProvider(),
		peermanagerProvider:  &mockPeerManagerProvider{},
		treeManager:          &mockTreeManager{},
		pool:                 &mockPool{},
		spaceService:         New(),
	}
	fx.app.Register(fx.account).
		Register(fx.config).
		Register(syncstatus.NewNoOpSyncStatus()).
		Register(credentialprovider.NewNoOp()).
		Register(fx.configurationService).
		Register(fx.storageProvider).
		Register(fx.peermanagerProvider).
		Register(fx.treeManager).
		Register(fx.pool).
		Register(fx.spaceService)
	err := fx.app.Start(ctx)
	if err != nil {
		fx.cancelFunc()
	}
	require.NoError(t, err)
	return fx
}

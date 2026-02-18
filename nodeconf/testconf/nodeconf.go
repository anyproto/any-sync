package testconf

import (
	"context"
	"time"

	"github.com/anyproto/go-chash"

	accountService "github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/nodeconf"
)

type StubConf struct {
	id            string
	networkId     string
	configuration nodeconf.Configuration

	// AdditionalNodes are extra nodes to register in the configuration
	// (in addition to the account's own node). Set before Init() is called.
	AdditionalNodes []nodeconf.Node
}

func (m *StubConf) NetworkCompatibilityStatus() nodeconf.NetworkCompatibilityStatus {
	return nodeconf.NetworkCompatibilityStatusOk
}

func (m *StubConf) Init(a *app.App) (err error) {
	accountKeys := a.MustComponent(accountService.CName).(accountService.Service).Account()
	networkId := accountKeys.SignKey.GetPublic().Network()
	node := nodeconf.Node{
		PeerId:    accountKeys.PeerId,
		Addresses: []string{"127.0.0.1:4430"},
		Types:     []nodeconf.NodeType{nodeconf.NodeTypeTree},
	}
	m.id = networkId
	m.networkId = networkId
	nodes := append([]nodeconf.Node{node}, m.AdditionalNodes...)
	m.configuration = nodeconf.Configuration{
		Id:           networkId,
		NetworkId:    networkId,
		Nodes:        nodes,
		CreationTime: time.Now(),
	}
	return nil
}

func (m *StubConf) Name() (name string) {
	return nodeconf.CName
}

func (m *StubConf) Run(ctx context.Context) (err error) {
	return nil
}

func (m *StubConf) Close(ctx context.Context) (err error) {
	return nil
}

func (m *StubConf) Id() string {
	return m.id
}

func (m *StubConf) Configuration() nodeconf.Configuration {
	return m.configuration
}

func (m *StubConf) NodeIds(spaceId string) []string {
	var nodeIds []string
	for _, node := range m.configuration.Nodes {
		nodeIds = append(nodeIds, node.PeerId)
	}
	return nodeIds
}

func (m *StubConf) IsResponsible(spaceId string) bool {
	return false
}

func (m *StubConf) FilePeers() []string {
	return nil
}

func (m *StubConf) ConsensusPeers() []string {
	return nil
}

func (m *StubConf) CoordinatorPeers() []string {
	return nil
}

func (m *StubConf) NamingNodePeers() []string {
	return nil
}

func (m *StubConf) PaymentProcessingNodePeers() []string {
	return nil
}

func (m *StubConf) PeerAddresses(peerId string) (addrs []string, ok bool) {
	for _, node := range m.configuration.Nodes {
		if node.PeerId == peerId {
			return node.Addresses, true
		}
	}
	return nil, false
}

func (m *StubConf) CHash() chash.CHash {
	return nil
}

func (m *StubConf) Partition(spaceId string) (part int) {
	return 0
}

func (m *StubConf) NodeTypes(nodeId string) []nodeconf.NodeType {
	for _, node := range m.configuration.Nodes {
		if node.PeerId == nodeId {
			return node.Types
		}
	}
	return nil
}

package nodeconf

import (
	"strings"

	"github.com/anyproto/go-chash"
)

type NodeConf interface {
	// Id returns current nodeconf id
	Id() string
	// Configuration returns configuration struct
	Configuration() Configuration
	// NodeIds returns list of peerId for given spaceId
	NodeIds(spaceId string) []string
	// IsResponsible checks if current account responsible for given spaceId
	IsResponsible(spaceId string) bool
	// FilePeers returns list of filenodes
	FilePeers() []string
	// ConsensusPeers returns list of consensusnodes
	ConsensusPeers() []string
	// CoordinatorPeers returns list of coordinator nodes
	CoordinatorPeers() []string
	// Please see any-ns-node repo for details
	// Usually one network has only 1 naming node, but we support array of NNs
	NamingNodePeers() []string
	// Please see any-pp-node repo for details
	PaymentProcessingNodePeers() []string
	// PeerAddresses returns peer addresses by peer id
	PeerAddresses(peerId string) (addrs []string, ok bool)
	// CHash returns nodes consistent table
	CHash() chash.CHash
	// Partition returns partition number by spaceId
	Partition(spaceId string) (part int)
	// NodeTypes returns list of known nodeTypes by nodeId, if node not registered in configuration will return empty list
	NodeTypes(nodeId string) []NodeType
}

type nodeConf struct {
	id                         string
	accountId                  string
	filePeers                  []string
	consensusPeers             []string
	coordinatorPeers           []string
	namingNodePeers            []string
	paymentProcessingNodePeers []string
	chash                      chash.CHash
	allMembers                 []Node
	c                          Configuration
	addrs                      map[string][]string
}

func (c *nodeConf) Id() string {
	return c.id
}

func (c *nodeConf) Configuration() Configuration {
	return c.c
}

func (c *nodeConf) NodeIds(spaceId string) []string {
	members := c.chash.GetMembers(ReplKey(spaceId))
	res := make([]string, 0, len(members))
	for _, m := range members {
		if m.Id() != c.accountId {
			res = append(res, m.Id())
		}
	}
	return res
}

func (c *nodeConf) IsResponsible(spaceId string) bool {
	for _, m := range c.chash.GetMembers(ReplKey(spaceId)) {
		if m.Id() == c.accountId {
			return true
		}
	}
	return false
}

func (c *nodeConf) FilePeers() []string {
	return c.filePeers
}

func (c *nodeConf) ConsensusPeers() []string {
	return c.consensusPeers
}

func (c *nodeConf) CoordinatorPeers() []string {
	return c.coordinatorPeers
}

func (c *nodeConf) NamingNodePeers() []string {
	return c.namingNodePeers
}

func (c *nodeConf) PaymentProcessingNodePeers() []string {
	return c.paymentProcessingNodePeers
}

func (c *nodeConf) PeerAddresses(peerId string) (addrs []string, ok bool) {
	addrs, ok = c.addrs[peerId]
	if ok && len(addrs) == 0 {
		return nil, false
	}
	return
}

func (c *nodeConf) CHash() chash.CHash {
	return c.chash
}

func (c *nodeConf) Partition(spaceId string) (part int) {
	return c.chash.GetPartition(ReplKey(spaceId))
}

func (c *nodeConf) NodeTypes(nodeId string) []NodeType {
	for _, m := range c.allMembers {
		if m.PeerId == nodeId {
			return m.Types
		}
	}
	return nil
}

func ReplKey(spaceId string) (replKey string) {
	if i := strings.LastIndex(spaceId, "."); i != -1 {
		return spaceId[i+1:]
	}
	return spaceId
}

func ConfigurationToNodeConf(c Configuration) (nc *nodeConf, err error) {
	nc = &nodeConf{
		id: c.Id,
		c:  c,

		// WARN: do not forget to set it later, Configuration does not feature it
		//accountId: s.accountId,

		addrs: map[string][]string{},
	}
	if nc.chash, err = chash.New(chash.Config{
		PartitionCount:    PartitionCount,
		ReplicationFactor: ReplicationFactor,
	}); err != nil {
		return
	}

	members := make([]chash.Member, 0, len(c.Nodes))
	for _, n := range c.Nodes {
		if n.HasType(NodeTypeTree) {
			members = append(members, n)
		}
		if n.HasType(NodeTypeConsensus) {
			nc.consensusPeers = append(nc.consensusPeers, n.PeerId)
		}
		if n.HasType(NodeTypeFile) {
			nc.filePeers = append(nc.filePeers, n.PeerId)
		}
		if n.HasType(NodeTypeCoordinator) {
			nc.coordinatorPeers = append(nc.coordinatorPeers, n.PeerId)
		}
		if n.HasType(NodeTypeNamingNode) {
			nc.namingNodePeers = append(nc.namingNodePeers, n.PeerId)
		}
		if n.HasType(NodeTypePaymentProcessingNode) {
			nc.paymentProcessingNodePeers = append(nc.paymentProcessingNodePeers, n.PeerId)
		}

		nc.allMembers = append(nc.allMembers, n)
		nc.addrs[n.PeerId] = n.Addresses
	}
	err = nc.chash.AddMembers(members...)
	return
}

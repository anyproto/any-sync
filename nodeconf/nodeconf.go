//go:generate mockgen -destination mock_nodeconf/mock_nodeconf.go github.com/anytypeio/any-sync/nodeconf Service
package nodeconf

import (
	"github.com/anytypeio/go-chash"
	"strings"
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
	// Addresses returns map[peerId][]addr with connection addresses for all known nodes
	Addresses() map[string][]string
	// CHash returns nodes consistent table
	CHash() chash.CHash
	// Partition returns partition number by spaceId
	Partition(spaceId string) (part int)
	// NodeTypes returns list of known nodeTypes by nodeId, if node not registered in configuration will return empty list
	NodeTypes(nodeId string) []NodeType
}

type nodeConf struct {
	id               string
	accountId        string
	filePeers        []string
	consensusPeers   []string
	coordinatorPeers []string
	chash            chash.CHash
	allMembers       []Node
	c                Configuration
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

func (c *nodeConf) Addresses() map[string][]string {
	res := make(map[string][]string)
	for _, m := range c.allMembers {
		res[m.PeerId] = m.Addresses
	}
	return res
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

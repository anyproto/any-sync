package nodeconf

import (
	"errors"
	"time"
)

type ConfigGetter interface {
	GetNodeConf() Configuration
}

type ConfigUpdateGetter interface {
	GetNodeConfUpdateInterval() int
}

var (
	ErrConfigurationNotFound = errors.New("node nodeConf not found")
)

type NodeType string

const (
	NodeTypeTree      NodeType = "tree"
	NodeTypeConsensus NodeType = "consensus"
	NodeTypeFile      NodeType = "file"

	NodeTypeCoordinator           NodeType = "coordinator"
	NodeTypeNamingNode            NodeType = "namingNode"
	NodeTypePaymentProcessingNode NodeType = "paymentProcessingNode"
)

type Node struct {
	PeerId    string     `yaml:"peerId" bson:"peerId"`
	Addresses []string   `yaml:"addresses" bson:"addresses"`
	Types     []NodeType `yaml:"types,omitempty" bson:"types"`
}

func (n Node) Id() string {
	return n.PeerId
}

func (n Node) Capacity() float64 {
	return 1
}

func (n Node) HasType(t NodeType) bool {
	for _, nt := range n.Types {
		if nt == t {
			return true
		}
	}
	return false
}

type Configuration struct {
	Id           string    `yaml:"id"`
	NetworkId    string    `yaml:"networkId"`
	Nodes        []Node    `yaml:"nodes"`
	CreationTime time.Time `yaml:"creationTime"`
}

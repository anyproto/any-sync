package nodeconf

type NodeType string

const (
	NodeTypeTree      NodeType = "tree"
	NodeTypeConsensus NodeType = "consensus"
	NodeTypeFile      NodeType = "file"

	NodeTypeCoordinator NodeType = "coordinator"
)

type ConfigGetter interface {
	GetNodes() []NodeConfig
	GetNodesConfId() string
}

type NodeConfig struct {
	PeerId        string     `yaml:"peerId"`
	Addresses     []string   `yaml:"address"`
	EncryptionKey string     `yaml:"encryptionPubKey,omitempty"`
	Types         []NodeType `yaml:"types,omitempty"`
}

func (n NodeConfig) HasType(t NodeType) bool {
	for _, nt := range n.Types {
		if nt == t {
			return true
		}
	}
	return false
}

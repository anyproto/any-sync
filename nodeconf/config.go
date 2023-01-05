package nodeconf

type NodeType string

const (
	NodeTypeTree      NodeType = "tree"
	NodeTypeConsensus NodeType = "consensus"
	NodeTypeFile      NodeType = "file"
)

type configGetter interface {
	GetNodes() []NodeConfig
}

type NodeConfig struct {
	PeerId        string     `yaml:"peerId"`
	Addresses     []string   `yaml:"address"`
	SigningKey    string     `yaml:"signingKey,omitempty"`
	EncryptionKey string     `yaml:"encryptionKey,omitempty"`
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

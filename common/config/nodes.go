package config

type NodeType string

const (
	NodeTypeTree      NodeType = "tree"
	NodeTypeConsensus NodeType = "consensus"
	NodeTypeFile      NodeType = "file"
)

type Node struct {
	PeerId        string     `yaml:"peerId"`
	Address       string     `yaml:"address"`
	SigningKey    string     `yaml:"signingKey,omitempty"`
	EncryptionKey string     `yaml:"encryptionKey,omitempty"`
	Types         []NodeType `yaml:"types,omitempty"`
}

func (n Node) HasType(t NodeType) bool {
	for _, nt := range n.Types {
		if nt == t {
			return true
		}
	}
	return false
}

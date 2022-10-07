package config

type Node struct {
	PeerId        string `yaml:"peerId"`
	Address       string `yaml:"address"`
	SigningKey    string `yaml:"signingKey,omitempty"`
	EncryptionKey string `yaml:"encryptionKey,omitempty"`
	IsConsensus   bool   `yaml:"isConsensus,omitempty"`
}

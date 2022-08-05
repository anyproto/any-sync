package config

type Node struct {
	PeerId        string `yaml:"peerId"`
	Address       string `yaml:"address"`
	SigningKey    string `yaml:"signingKey"`
	EncryptionKey string `yaml:"encryptionKey"`
}

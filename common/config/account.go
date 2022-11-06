package config

type Account struct {
	PeerId        string `yaml:"peerId"`
	PeerKey       string `yaml:"peerKey"`
	SigningKey    string `yaml:"signingKey"`
	EncryptionKey string `yaml:"encryptionKey"`
}

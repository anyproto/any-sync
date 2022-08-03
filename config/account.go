package config

type Account struct {
	PeerId        string `yaml:"peerId"`
	SigningKey    string `yaml:"signingKey"`
	EncryptionKey string `yaml:"encryptionKey"`
}

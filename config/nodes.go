package config

type Node struct {
	Alias         string `yaml:"alias"`
	Address       string `yaml:"address"`
	SigningKey    string `yaml:"signingKey"`
	EncryptionKey string `yaml:"encryptionKey"`
}

type NodeInfo struct {
	CurrentAlias string  `yaml:"currentAlias"`
	Nodes        []*Node `yaml:"nodes"`
}

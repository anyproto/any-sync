package net

type ConfigGetter interface {
	GetNet() Config
}

type Config struct {
	Server ServerConfig `yaml:"server"`
	Stream StreamConfig `yaml:"stream"`
}

type ServerConfig struct {
	ListenAddrs []string `yaml:"listenAddrs"`
}

type StreamConfig struct {
	TimeoutMilliseconds int `yaml:"timeoutMilliseconds"`
	MaxMsgSizeMb        int `yaml:"maxMsgSizeMb"`
}

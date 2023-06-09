package rpc

type ConfigGetter interface {
	GetDrpc() Config
}

type Config struct {
	Stream StreamConfig `yaml:"stream"`
}

type StreamConfig struct {
	MaxMsgSizeMb int `yaml:"maxMsgSizeMb"`
}

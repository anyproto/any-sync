package config

type Stream struct {
	TimeoutMilliseconds int `yaml:"timeoutMilliseconds"`
	MaxMsgSizeMb        int `yaml:"maxMsgSizeMb"`
}

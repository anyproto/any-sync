package net

import "errors"

var (
	ErrUnableToConnect = errors.New("unable to connect")
)

type ConfigGetter interface {
	GetNet() Config
}

type Config struct {
	Stream StreamConfig `yaml:"stream"`
}

type StreamConfig struct {
	TimeoutMilliseconds int `yaml:"timeoutMilliseconds"`
	MaxMsgSizeMb        int `yaml:"maxMsgSizeMb"`
}

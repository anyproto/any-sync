package config

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"gopkg.in/yaml.v3"
	"io/ioutil"
)

const CName = "config"

func NewFromFile(path string) (c *Config, err error) {
	c = &Config{}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err = yaml.Unmarshal(data, c); err != nil {
		return nil, err
	}
	return
}

type Config struct {
	Anytype    Anytype    `yaml:"anytype"`
	GrpcServer GrpcServer `yaml:"grpcServer"`
	PeerList   PeerList   `yaml:"peerList"`
}

func (c *Config) Init(ctx context.Context, a *app.App) (err error) {
	logger.NewNamed("config").Info(fmt.Sprint(*c))
	return
}

func (c Config) Name() (name string) {
	return CName
}

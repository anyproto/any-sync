package config

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
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
	GrpcServer GrpcServer `yaml:"grpcServer"`
	Account    Account    `yaml:"account"`
	Mongo      Mongo      `yaml:"mongo"`
}

func (c *Config) Init(a *app.App) (err error) {
	return
}

func (c Config) Name() (name string) {
	return CName
}

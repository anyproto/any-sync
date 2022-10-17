package config

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
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
	GrpcServer config.GrpcServer `yaml:"grpcServer"`
	Account    config.Account    `yaml:"account"`
	Mongo      Mongo             `yaml:"mongo"`
	Metric     config.Metric     `yaml:"metric"`
	Log        config.Log        `yaml:"log"`
}

func (c *Config) Init(a *app.App) (err error) {
	return
}

func (c Config) Name() (name string) {
	return CName
}

func (c Config) GetMongo() Mongo {
	return c.Mongo
}

func (c Config) GetGRPCServer() config.GrpcServer {
	return c.GrpcServer
}

func (c Config) GetAccount() config.Account {
	return c.Account
}

func (c Config) GetMetric() config.Metric {
	return c.Metric
}

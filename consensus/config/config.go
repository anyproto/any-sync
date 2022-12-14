package config

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	config2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
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
	GrpcServer config2.GrpcServer `yaml:"grpcServer"`
	Account    config2.Account    `yaml:"account"`
	Mongo      Mongo              `yaml:"mongo"`
	Metric     config2.Metric     `yaml:"metric"`
	Log        config2.Log        `yaml:"log"`
	Stream     config2.Stream     `yaml:"stream"`
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

func (c Config) GetGRPCServer() config2.GrpcServer {
	return c.GrpcServer
}

func (c Config) GetStream() config2.Stream {
	return c.Stream
}

func (c Config) GetAccount() config2.Account {
	return c.Account
}

func (c Config) GetMetric() config2.Metric {
	return c.Metric
}

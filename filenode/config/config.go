package config

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
	"gopkg.in/yaml.v3"
	"io/ioutil"
)

const CName = config.CName

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
	Account         config.Account         `yaml:"account"`
	GrpcServer      config.GrpcServer      `yaml:"grpcServer"`
	Metric          config.Metric          `yaml:"metric"`
	FileStorePogreb config.FileStorePogreb `yaml:"fileStorePogreb"`
}

func (c *Config) Init(a *app.App) (err error) {
	return
}

func (c Config) Name() (name string) {
	return CName
}

func (c Config) GetAccount() config.Account {
	return c.Account
}

func (c Config) GetFileStorePogreb() config.FileStorePogreb {
	return c.FileStorePogreb
}

func (c Config) GetGRPCServer() config.GrpcServer {
	return c.GrpcServer
}

func (c Config) GetMetric() config.Metric {
	return c.Metric
}

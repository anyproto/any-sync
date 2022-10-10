package config

import (
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
	Account    Account    `yaml:"account"`
	APIServer  APIServer  `yaml:"apiServer"`
	Nodes      []Node     `yaml:"nodes"`
	Space      Space      `yaml:"space"`
	Metric     Metric     `yaml:"metric"`
	Log        Log        `yaml:"log"`
}

func (c *Config) Init(a *app.App) (err error) {
	logger.NewNamed("config").Info(fmt.Sprint(c.Space))
	return
}

func (c Config) Name() (name string) {
	return CName
}

func (c Config) GetAnytype() Anytype {
	return c.Anytype
}

func (c Config) GetGRPCServer() GrpcServer {
	return c.GrpcServer
}

func (c Config) GetAccount() Account {
	return c.Account
}

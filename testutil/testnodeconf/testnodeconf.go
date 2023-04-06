package testnodeconf

import (
	"github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/nodeconf/nodeconfstore"
	"github.com/anytypeio/any-sync/testutil/accounttest"
)

func GenNodeConfig(num int) (conf *Config) {
	conf = &Config{}
	if num <= 0 {
		num = 1
	}
	for i := 0; i < num; i++ {
		ac := &accounttest.AccountTestService{}
		if err := ac.Init(nil); err != nil {
			panic(err)
		}
		conf.nodes = append(conf.nodes, ac.NodeConf(nil))
		conf.configs = append(conf.configs, ac)
	}
	return conf
}

type Config struct {
	nodes   []nodeconfstore.NodeConfig
	configs []*accounttest.AccountTestService
}

func (c *Config) Init(a *app.App) (err error) { return }
func (c *Config) Name() string                { return "config" }

func (c *Config) GetNodesConfId() string {
	return "test"
}

func (c *Config) GetNodes() []nodeconfstore.NodeConfig {
	return c.nodes
}

func (c *Config) GetAccountService(idx int) accountservice.Service {
	return c.configs[idx]
}

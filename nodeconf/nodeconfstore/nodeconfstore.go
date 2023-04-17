package nodeconfstore

import (
	"context"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/nodeconf"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"sync"
)

func New() NodeConfStore {
	return new(nodeConfStore)
}

type NodeConfStore interface {
	app.Component
	nodeconf.Store
}

type nodeConfStore struct {
	path string
	mu   sync.Mutex
}

type configGetter interface {
	GetNodeConfStorePath() string
}

func (n *nodeConfStore) Init(a *app.App) (err error) {
	n.path = a.MustComponent("config").(configGetter).GetNodeConfStorePath()
	if e := os.Mkdir(n.path, 0755); e != nil && !os.IsExist(e) {
		return e
	}
	return
}

func (n *nodeConfStore) Name() (name string) {
	return nodeconf.CNameStore
}

func (n *nodeConfStore) GetLast(ctx context.Context, netId string) (c nodeconf.Configuration, err error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	path := filepath.Join(n.path, netId+".yml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		err = nodeconf.ErrConfigurationNotFound
		return
	}
	err = yaml.Unmarshal(data, &c)
	return
}

func (n *nodeConfStore) SaveLast(ctx context.Context, c nodeconf.Configuration) (err error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	path := filepath.Join(n.path, c.NetworkId+".yml")
	data, err := yaml.Marshal(c)
	if err != nil {
		return
	}
	return os.WriteFile(path, data, 0755)
}

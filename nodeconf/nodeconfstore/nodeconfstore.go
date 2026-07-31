package nodeconfstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/nodeconf"
	"gopkg.in/yaml.v3"
)

// historyLimit is the number of historical configurations retained per network.
const historyLimit = 100

func New() NodeConfStore {
	return new(nodeConfStore)
}

type NodeConfStore interface {
	app.Component
	nodeconf.HistoryStore
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
	if e := os.MkdirAll(n.path, 0o755); e != nil && !os.IsExist(e) {
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
	if err = os.WriteFile(path, data, 0o644); err != nil {
		return
	}
	if c.Epoch > 0 {
		if err = os.WriteFile(n.epochPath(c.NetworkId, c.Epoch), data, 0o644); err != nil {
			return
		}
		err = n.pruneHistory(c.NetworkId)
	}
	return
}

func (n *nodeConfStore) GetByEpoch(ctx context.Context, netId string, epoch uint64) (c nodeconf.Configuration, err error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	data, err := os.ReadFile(n.epochPath(netId, epoch))
	if os.IsNotExist(err) {
		err = nodeconf.ErrConfigurationNotFound
		return
	}
	if err != nil {
		return
	}
	err = yaml.Unmarshal(data, &c)
	return
}

func (n *nodeConfStore) Epochs(ctx context.Context, netId string) (epochs []uint64, err error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.epochs(netId)
}

func (n *nodeConfStore) epochPath(netId string, epoch uint64) string {
	return filepath.Join(n.path, fmt.Sprintf("%s.e%d.yml", netId, epoch))
}

func (n *nodeConfStore) epochs(netId string) (epochs []uint64, err error) {
	entries, err := os.ReadDir(n.path)
	if err != nil {
		return
	}
	prefix := netId + ".e"
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".yml") {
			continue
		}
		epoch, pErr := strconv.ParseUint(strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".yml"), 10, 64)
		if pErr != nil {
			continue
		}
		epochs = append(epochs, epoch)
	}
	sort.Slice(epochs, func(i, j int) bool { return epochs[i] < epochs[j] })
	return
}

func (n *nodeConfStore) pruneHistory(netId string) (err error) {
	epochs, err := n.epochs(netId)
	if err != nil {
		return
	}
	for len(epochs) > historyLimit {
		if err = os.Remove(n.epochPath(netId, epochs[0])); err != nil {
			return
		}
		epochs = epochs[1:]
	}
	return
}

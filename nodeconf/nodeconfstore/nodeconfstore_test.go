package nodeconfstore

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var ctx = context.Background()

func TestNodeConfStore_GetLast(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		_, err := fx.GetLast(ctx, "123")
		assert.EqualError(t, err, nodeconf.ErrConfigurationNotFound.Error())
	})
	t.Run("success", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		c := nodeconf.Configuration{
			Id:        "123",
			NetworkId: "456",
			Nodes: []nodeconf.Node{
				{
					PeerId:    "peerId",
					Addresses: []string{"addr1", "addr2"},
					Types:     []nodeconf.NodeType{nodeconf.NodeTypeTree, nodeconf.NodeTypeCoordinator},
				},
			},
			CreationTime: time.Now().Round(time.Second),
		}
		require.NoError(t, fx.SaveLast(ctx, c))

		res, err := fx.GetLast(ctx, "456")
		require.NoError(t, err)
		assert.Equal(t, c.CreationTime.Unix(), res.CreationTime.Unix())
		c.CreationTime = res.CreationTime
		assert.Equal(t, c, res)
	})
}

type fixture struct {
	NodeConfStore
	tmpPath string
	a       *app.App
}

func newFixture(t *testing.T) *fixture {
	fx := &fixture{
		NodeConfStore: New(),
		a:             new(app.App),
	}
	var err error
	fx.tmpPath, err = os.MkdirTemp("", "")
	require.NoError(t, err)
	fx.a.Register(config{path: fx.tmpPath}).Register(fx.NodeConfStore)
	require.NoError(t, fx.a.Start(ctx))
	return fx
}

func (fx *fixture) finish(t *testing.T) {
	defer os.RemoveAll(fx.tmpPath)
	require.NoError(t, fx.a.Close(ctx))
}

type config struct {
	path string
}

func (c config) GetNodeConfStorePath() string {
	return filepath.Join(c.path, "nodeconf")
}

func (c config) Init(a *app.App) (err error) {
	return
}

func (c config) Name() (name string) {
	return "config"
}

func TestNodeConfStore_History(t *testing.T) {
	t.Run("get by epoch", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		for epoch := uint64(1); epoch <= 3; epoch++ {
			c := nodeconf.Configuration{
				Id:        "id" + string(rune('0'+epoch)),
				NetworkId: "net1",
				Epoch:     epoch,
			}
			require.NoError(t, fx.SaveLast(ctx, c))
		}

		c, err := fx.GetByEpoch(ctx, "net1", 2)
		require.NoError(t, err)
		assert.Equal(t, "id2", c.Id)
		assert.Equal(t, uint64(2), c.Epoch)

		last, err := fx.GetLast(ctx, "net1")
		require.NoError(t, err)
		assert.Equal(t, uint64(3), last.Epoch)

		epochs, err := fx.Epochs(ctx, "net1")
		require.NoError(t, err)
		assert.Equal(t, []uint64{1, 2, 3}, epochs)

		_, err = fx.GetByEpoch(ctx, "net1", 10)
		assert.EqualError(t, err, nodeconf.ErrConfigurationNotFound.Error())
	})
	t.Run("no history for epochless configs", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		require.NoError(t, fx.SaveLast(ctx, nodeconf.Configuration{Id: "1", NetworkId: "net2"}))
		epochs, err := fx.Epochs(ctx, "net2")
		require.NoError(t, err)
		assert.Empty(t, epochs)
	})
	t.Run("prune", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		for epoch := uint64(1); epoch <= historyLimit+3; epoch++ {
			require.NoError(t, fx.SaveLast(ctx, nodeconf.Configuration{Id: "x", NetworkId: "net3", Epoch: epoch}))
		}
		epochs, err := fx.Epochs(ctx, "net3")
		require.NoError(t, err)
		require.Len(t, epochs, historyLimit)
		assert.Equal(t, uint64(4), epochs[0])
		assert.Equal(t, uint64(historyLimit+3), epochs[len(epochs)-1])
	})
}

package nodeconfstore

import (
	"context"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
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
	return c.path
}

func (c config) Init(a *app.App) (err error) {
	return
}

func (c config) Name() (name string) {
	return "config"
}

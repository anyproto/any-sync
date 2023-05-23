package nodeconf

import (
	"context"
	"errors"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/net"
	"github.com/anytypeio/any-sync/net/secureservice/handshake"
	"github.com/anytypeio/any-sync/testutil/accounttest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

var ctx = context.Background()

func TestService_NetworkCompatibilityStatus(t *testing.T) {
	t.Run("unknown", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.testSource.call = func() (c Configuration, e error) {
			e = net.ErrUnableToConnect
			return
		}
		fx.run(t)
		time.Sleep(time.Millisecond * 10)
		assert.Equal(t, NetworkCompatibilityStatusUnknown, fx.NetworkCompatibilityStatus())
	})
	t.Run("incompatible", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.testSource.err = handshake.ErrIncompatibleVersion
		fx.run(t)
		time.Sleep(time.Millisecond * 10)
		assert.Equal(t, NetworkCompatibilityStatusIncompatible, fx.NetworkCompatibilityStatus())
	})
	t.Run("error", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.testSource.call = func() (c Configuration, e error) {
			e = errors.New("some error")
			return
		}
		fx.run(t)
		time.Sleep(time.Millisecond * 10)
		assert.Equal(t, NetworkCompatibilityStatusError, fx.NetworkCompatibilityStatus())
	})
	t.Run("ok", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.run(t)
		time.Sleep(time.Millisecond * 10)
		assert.Equal(t, NetworkCompatibilityStatusOk, fx.NetworkCompatibilityStatus())
	})
}

func newFixture(t *testing.T) *fixture {
	fx := &fixture{
		Service:    New(),
		a:          new(app.App),
		testStore:  &testStore{},
		testSource: &testSource{},
		testConf:   newTestConf(),
	}
	fx.a.Register(fx.testConf).Register(&accounttest.AccountTestService{}).Register(fx.Service).Register(fx.testSource).Register(fx.testStore)
	return fx
}

type fixture struct {
	Service
	a          *app.App
	testStore  *testStore
	testSource *testSource
	testConf   *testConf
}

func (fx *fixture) run(t *testing.T) {
	require.NoError(t, fx.a.Start(ctx))
}

func (fx *fixture) finish(t *testing.T) {
	require.NoError(t, fx.a.Close(ctx))
}

type testSource struct {
	conf Configuration
	err  error
	call func() (Configuration, error)
}

func (t *testSource) Init(a *app.App) error { return nil }
func (t *testSource) Name() string          { return CNameSource }

func (t *testSource) GetLast(ctx context.Context, currentId string) (c Configuration, err error) {
	if t.call != nil {
		return t.call()
	}
	return t.conf, t.err
}

type testStore struct {
	conf *Configuration
	mu   sync.Mutex
}

func (t *testStore) Init(a *app.App) error { return nil }
func (t *testStore) Name() string          { return CNameStore }

func (t *testStore) GetLast(ctx context.Context, netId string) (c Configuration, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conf != nil {
		return *t.conf, nil
	} else {
		err = ErrConfigurationNotFound
	}
	return
}

func (t *testStore) SaveLast(ctx context.Context, c Configuration) (err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.conf = &c
	return
}

type testConf struct {
	Configuration
}

func (t *testConf) Init(a *app.App) error { return nil }
func (t *testConf) Name() string          { return "config" }

func (t *testConf) GetNodeConf() Configuration {
	return t.Configuration
}

func newTestConf() *testConf {
	return &testConf{
		Configuration{
			Id:        "test",
			NetworkId: "testNetwork",
			Nodes: []Node{
				{
					PeerId:    "12D3KooWKLCajM89S8unbt3tgGbRLgmiWnFZT3adn9A5pQciBSLa",
					Addresses: []string{"127.0.0.1:4830"},
					Types:     []NodeType{NodeTypeCoordinator},
				},
				{
					PeerId:    "12D3KooWKnXTtbveMDUFfeSqR5dt9a4JW66tZQXG7C7PdDh3vqGu",
					Addresses: []string{"127.0.0.1:4730"},
					Types:     []NodeType{NodeTypeTree},
				},
				{
					PeerId:    "12D3KooWKgVN2kW8xw5Uvm2sLUnkeUNQYAvcWvF58maTzev7FjPi",
					Addresses: []string{"127.0.0.1:4731"},
					Types:     []NodeType{NodeTypeTree},
				},
				{
					PeerId:    "12D3KooWCUPYuMnQhu9yREJgQyjcz8zWY83rZGmDLwb9YR6QkbZX",
					Addresses: []string{"127.0.0.1:4732"},
					Types:     []NodeType{NodeTypeTree},
				},
				{
					PeerId:    "12D3KooWQxiZ5a7vcy4DTJa8Gy1eVUmwb5ojN4SrJC9Rjxzigw6C",
					Addresses: []string{"127.0.0.1:4733"},
					Types:     []NodeType{NodeTypeFile},
				},
			},
			CreationTime: time.Now(),
		},
	}
}

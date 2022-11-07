package rpctest

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/pool"
	"math/rand"
	"storj.io/drpc"
	"sync"
	"time"
)

var ErrCantConnect = errors.New("can't connect to test server")

func NewTestPool() *TestPool {
	return &TestPool{}
}

type TestPool struct {
	ts *TesServer
	mu sync.Mutex
}

func (t *TestPool) WithServer(ts *TesServer) *TestPool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ts = ts
	return t
}

func (t *TestPool) Get(ctx context.Context, id string) (peer.Peer, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ts == nil {
		return nil, ErrCantConnect
	}
	return &testPeer{id: id, Conn: t.ts.Dial(ctx)}, nil
}

func (t *TestPool) Dial(ctx context.Context, id string) (peer.Peer, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ts == nil {
		return nil, ErrCantConnect
	}
	return &testPeer{id: id, Conn: t.ts.Dial(ctx)}, nil
}

func (t *TestPool) GetOneOf(ctx context.Context, peerIds []string) (peer.Peer, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ts == nil {
		return nil, ErrCantConnect
	}
	return &testPeer{id: peerIds[rand.Intn(len(peerIds))], Conn: t.ts.Dial(ctx)}, nil
}

func (t *TestPool) DialOneOf(ctx context.Context, peerIds []string) (peer.Peer, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ts == nil {
		return nil, ErrCantConnect
	}
	return &testPeer{id: peerIds[rand.Intn(len(peerIds))], Conn: t.ts.Dial(ctx)}, nil
}

func (t *TestPool) Init(a *app.App) (err error) {
	return nil
}

func (t *TestPool) Name() (name string) {
	return pool.CName
}

func (t *TestPool) Run(ctx context.Context) (err error) {
	return nil
}

func (t *TestPool) Close(ctx context.Context) (err error) {
	return nil
}

type testPeer struct {
	id string
	drpc.Conn
}

func (t testPeer) Id() string {
	return t.id
}

func (t testPeer) LastUsage() time.Time {
	return time.Now()
}

func (t testPeer) UpdateLastUsage() {}

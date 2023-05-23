package rpctest

import (
	"context"
	"errors"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"math/rand"
	"storj.io/drpc"
	"sync"
	"time"
)

var ErrCantConnect = errors.New("can't connect to test server")

func NewTestPool() *TestPool {
	return &TestPool{
		peers: map[string]peer.Peer{},
	}
}

type TestPool struct {
	ts    *TesServer
	peers map[string]peer.Peer
	mu    sync.Mutex
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
	if p, ok := t.peers[id]; ok {
		return p, nil
	}
	if t.ts == nil {
		return nil, ErrCantConnect
	}
	return &testPeer{id: id, Conn: t.ts.Dial(ctx)}, nil
}

func (t *TestPool) Dial(ctx context.Context, id string) (peer.Peer, error) {
	return t.Get(ctx, id)
}

func (t *TestPool) GetOneOf(ctx context.Context, peerIds []string) (peer.Peer, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, peerId := range peerIds {
		if p, ok := t.peers[peerId]; ok {
			return p, nil
		}
	}
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

func (t *TestPool) NewPool(name string) pool.Pool {
	return t
}

func (t *TestPool) AddPeer(p peer.Peer) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.peers[p.Id()] = p
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

func (t testPeer) Addr() string {
	return ""
}

func (t testPeer) TryClose(objectTTL time.Duration) (res bool, err error) {
	return true, t.Close()
}

func (t testPeer) Id() string {
	return t.id
}

func (t testPeer) LastUsage() time.Time {
	return time.Now()
}

func (t testPeer) UpdateLastUsage() {}

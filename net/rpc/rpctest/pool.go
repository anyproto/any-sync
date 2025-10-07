package rpctest

import (
	"context"
	"fmt"
	"sync"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
)

func NewTestPool() *TestPool {
	return &TestPool{peers: map[string]peer.Peer{}}
}

type TestPool struct {
	peers map[string]peer.Peer
	mu    sync.Mutex
	ts    *TestServer
}

func (t *TestPool) Flush(ctx context.Context) error {
	return nil
}

func (t *TestPool) Init(a *app.App) (err error) {
	return nil
}

func (t *TestPool) Name() (name string) {
	return pool.CName
}

func (t *TestPool) WithServer(ts *TestServer) *TestPool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ts = ts
	return t
}

func (t *TestPool) Run(ctx context.Context) (err error) {
	return nil
}

func (t *TestPool) Close(ctx context.Context) (err error) {
	return nil
}

func (t *TestPool) Get(ctx context.Context, id string) (peer.Peer, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if p, ok := t.peers[id]; ok {
		return p, nil
	}
	if t.ts == nil {
		return nil, net.ErrUnableToConnect
	}
	return t.ts.Dial(id)
}

func (t *TestPool) GetOneOf(ctx context.Context, peerIds []string) (peer.Peer, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, peerId := range peerIds {
		if p, ok := t.peers[peerId]; ok {
			return p, nil
		}
	}
	if t.ts == nil || len(peerIds) == 0 {
		return nil, net.ErrUnableToConnect
	}
	return t.ts.Dial(peerIds[0])
}

func (t *TestPool) AddPeer(ctx context.Context, p peer.Peer) (err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.peers[p.Id()] = p
	return nil
}

func (t *TestPool) Pick(ctx context.Context, id string) (peer.Peer, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if p, ok := t.peers[id]; ok {
		return p, nil
	}
	if t.ts == nil {
		return nil, net.ErrUnableToConnect
	}
	return nil, fmt.Errorf("not found")
}

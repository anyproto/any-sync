package rpctest

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"sync"
)

func NewTestPool() *TestPool {
	return &TestPool{peers: map[string]peer.Peer{}}
}

type TestPool struct {
	peers map[string]peer.Peer
	mu    sync.Mutex
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

func (t *TestPool) Get(ctx context.Context, id string) (peer.Peer, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if p, ok := t.peers[id]; ok {
		return p, nil
	}
	return nil, net.ErrUnableToConnect
}

func (t *TestPool) GetOneOf(ctx context.Context, peerIds []string) (peer.Peer, error) {
	for _, id := range peerIds {
		if p, err := t.Get(ctx, id); err == nil {
			return p, nil
		}
	}
	return nil, net.ErrUnableToConnect
}

func (t *TestPool) AddPeer(ctx context.Context, p peer.Peer) (err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.peers[p.Id()] = p
	return nil
}

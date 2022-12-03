package peers

import (
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
)

type PeerType int

const (
	PeerTypeClient PeerType = iota
	PeerTypeNode
)

const CName = "debug.peers"

var ErrNoSuchPeer = errors.New("no such peer")

type Service interface {
	app.Component
	Get(alias string) (Peer, error)
}

type Peer struct {
	PeerType PeerType
	Address  string
}

type service struct {
	peerMap map[string]Peer
}

func New() Service {
	return &service{peerMap: map[string]Peer{}}
}

func (s *service) Init(a *app.App) (err error) {
	s.peerMap["client1"] = Peer{
		PeerType: PeerTypeClient,
		Address:  "127.0.0.1:8090",
	}
	s.peerMap["client2"] = Peer{
		PeerType: PeerTypeClient,
		Address:  "127.0.0.1:8091",
	}
	s.peerMap["node1"] = Peer{
		PeerType: PeerTypeNode,
		Address:  "127.0.0.1:8080",
	}
	return
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Get(alias string) (Peer, error) {
	peer, ok := s.peerMap[alias]
	if !ok {
		return Peer{}, ErrNoSuchPeer
	}
	return peer, nil
}

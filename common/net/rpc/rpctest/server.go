package rpctest

import (
	"context"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"net"
	"storj.io/drpc"
	"storj.io/drpc/drpcconn"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
)

type SecConnMock struct {
	net.Conn
	localPrivKey crypto.PrivKey
	remotePubKey crypto.PubKey
	localId      peer.ID
	remoteId     peer.ID
}

func (s *SecConnMock) LocalPeer() peer.ID {
	return s.localId
}

func (s *SecConnMock) LocalPrivateKey() crypto.PrivKey {
	return s.localPrivKey
}

func (s *SecConnMock) RemotePeer() peer.ID {
	return s.remoteId
}

func (s *SecConnMock) RemotePublicKey() crypto.PubKey {
	return s.remotePubKey
}

type ConnWrapper func(conn net.Conn) net.Conn

func NewSecConnWrapper(
	localPrivKey crypto.PrivKey,
	remotePubKey crypto.PubKey,
	localId peer.ID,
	remoteId peer.ID) ConnWrapper {
	return func(conn net.Conn) net.Conn {
		return &SecConnMock{
			Conn:         conn,
			localPrivKey: localPrivKey,
			remotePubKey: remotePubKey,
			localId:      localId,
			remoteId:     remoteId,
		}
	}
}

func NewTestServer() *TesServer {
	ts := &TesServer{
		Mux: drpcmux.New(),
	}
	ts.Server = drpcserver.New(ts.Mux)
	return ts
}

type TesServer struct {
	*drpcmux.Mux
	*drpcserver.Server
}

func (ts *TesServer) Dial() drpc.Conn {
	return ts.DialWrapConn(nil, nil)
}

func (ts *TesServer) DialWrapConn(serverWrapper ConnWrapper, clientWrapper ConnWrapper) drpc.Conn {
	sc, cc := net.Pipe()
	if serverWrapper != nil {
		sc = serverWrapper(sc)
	}
	if clientWrapper != nil {
		cc = clientWrapper(cc)
	}
	go ts.Server.ServeOne(context.Background(), sc)
	return drpcconn.New(cc)
}

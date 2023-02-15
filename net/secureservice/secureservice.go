package secureservice

import (
	"context"
	commonaccount "github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/commonspace/object/accountdata"
	"github.com/anytypeio/any-sync/net/secureservice/handshake"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/sec"
	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"
	"go.uber.org/zap"
	"net"
)

type HandshakeError struct {
	remoteAddr string
	err        error
}

func (he HandshakeError) RemoteAddr() string {
	return he.remoteAddr
}

func (he HandshakeError) Error() string {
	return he.err.Error()
}

const CName = "common.net.secure"

var log = logger.NewNamed(CName)

func New() SecureService {
	return &secureService{}
}

type SecureService interface {
	TLSListener(lis net.Listener, timeoutMillis int, withIdentityCheck bool) ContextListener
	BasicListener(lis net.Listener, timeoutMillis int) ContextListener
	TLSConn(ctx context.Context, conn net.Conn) (sec.SecureConn, error)
	app.Component
}

type secureService struct {
	outboundTr *libp2ptls.Transport
	account    *accountdata.AccountData
	key        crypto.PrivKey
	nodeconf   nodeconf.Service

	noVerifyChecker  handshake.CredentialChecker
	peerSignVerifier handshake.CredentialChecker
}

func (s *secureService) Init(a *app.App) (err error) {
	account := a.MustComponent(commonaccount.CName).(commonaccount.Service)
	peerKey, err := account.Account().PeerKey.Raw()
	if err != nil {
		return
	}
	if s.key, err = crypto.UnmarshalEd25519PrivateKey(peerKey); err != nil {
		return
	}

	s.noVerifyChecker = newNoVerifyChecker()
	s.peerSignVerifier = newPeerSignVerifier(account.Account())

	s.nodeconf = a.MustComponent(nodeconf.CName).(nodeconf.Service)

	if s.outboundTr, err = libp2ptls.New(libp2ptls.ID, s.key, nil); err != nil {
		return
	}

	log.Info("secure service init", zap.String("peerId", account.Account().PeerId))
	return nil
}

func (s *secureService) Name() (name string) {
	return CName
}

func (s *secureService) TLSListener(lis net.Listener, timeoutMillis int, identityHandshake bool) ContextListener {
	cc := s.noVerifyChecker
	if identityHandshake {
		cc = s.peerSignVerifier
	}
	return newTLSListener(cc, s.key, lis, timeoutMillis)
}

func (s *secureService) BasicListener(lis net.Listener, timeoutMillis int) ContextListener {
	return newBasicListener(lis, timeoutMillis)
}

func (s *secureService) TLSConn(ctx context.Context, conn net.Conn) (sec.SecureConn, error) {
	sc, err := s.outboundTr.SecureOutbound(ctx, conn, "")
	if err != nil {
		return nil, HandshakeError{err: err, remoteAddr: conn.RemoteAddr().String()}
	}
	peerId := sc.RemotePeer().String()
	confTypes := s.nodeconf.GetLast().NodeTypes(peerId)
	var checker handshake.CredentialChecker
	if len(confTypes) > 0 {
		checker = s.peerSignVerifier
	} else {
		checker = s.noVerifyChecker
	}
	// ignore identity for outgoing connection because we don't need it at this moment
	_, err = handshake.OutgoingHandshake(ctx, sc, checker)
	if err != nil {
		return nil, HandshakeError{err: err, remoteAddr: conn.RemoteAddr().String()}
	}
	return sc, nil
}

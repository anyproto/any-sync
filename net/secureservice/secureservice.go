package secureservice

import (
	"context"
	commonaccount "github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/commonspace/object/accountdata"
	"github.com/anytypeio/any-sync/net/peer"
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
	SecureOutbound(ctx context.Context, conn net.Conn) (sec.SecureConn, error)
	SecureInbound(ctx context.Context, conn net.Conn) (cctx context.Context, sc sec.SecureConn, err error)
	app.Component
}

type secureService struct {
	p2pTr        *libp2ptls.Transport
	account      *accountdata.AccountKeys
	key          crypto.PrivKey
	nodeconf     nodeconf.Service
	protoVersion uint32

	noVerifyChecker  handshake.CredentialChecker
	peerSignVerifier handshake.CredentialChecker
	inboundChecker   handshake.CredentialChecker
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
	s.noVerifyChecker = newNoVerifyChecker(s.protoVersion)
	s.peerSignVerifier = newPeerSignVerifier(s.protoVersion, account.Account())

	s.nodeconf = a.MustComponent(nodeconf.CName).(nodeconf.Service)

	s.inboundChecker = s.noVerifyChecker
	confTypes := s.nodeconf.GetLast().NodeTypes(account.Account().PeerId)
	if len(confTypes) > 0 {
		// require identity verification if we are node
		s.inboundChecker = s.peerSignVerifier
	}

	if s.p2pTr, err = libp2ptls.New(libp2ptls.ID, s.key, nil); err != nil {
		return
	}

	log.Info("secure service init", zap.String("peerId", account.Account().PeerId))
	return nil
}

func (s *secureService) Name() (name string) {
	return CName
}

func (s *secureService) SecureInbound(ctx context.Context, conn net.Conn) (cctx context.Context, sc sec.SecureConn, err error) {
	sc, err = s.p2pTr.SecureInbound(ctx, conn, "")
	if err != nil {
		return nil, nil, HandshakeError{
			remoteAddr: conn.RemoteAddr().String(),
			err:        err,
		}
	}

	identity, err := handshake.IncomingHandshake(ctx, sc, s.inboundChecker)
	if err != nil {
		return nil, nil, HandshakeError{
			remoteAddr: conn.RemoteAddr().String(),
			err:        err,
		}
	}
	cctx = context.Background()
	cctx = peer.CtxWithPeerId(cctx, sc.RemotePeer().String())
	cctx = peer.CtxWithIdentity(cctx, identity)
	return
}

func (s *secureService) SecureOutbound(ctx context.Context, conn net.Conn) (sec.SecureConn, error) {
	sc, err := s.p2pTr.SecureOutbound(ctx, conn, "")
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

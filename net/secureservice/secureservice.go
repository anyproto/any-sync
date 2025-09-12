package secureservice

import (
	"context"
	"crypto/tls"
	"io"
	"net"

	"github.com/libp2p/go-libp2p/core/crypto"
	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"
	"go.uber.org/zap"

	commonaccount "github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/nodeconf"
)

const CName = "common.net.secure"

var log = logger.NewNamed(CName)

var (
	// ProtoVersion 0 - first any-sync version with raw tcp connections
	// ProtoVersion 1 - version with yamux over tcp and quic
	// ProtoVersion 2 - acl compatible version
	// ProtoVersion 3 - acl with breaking changes / multiplayer
	// ProtoVersion 4 - new sync compatible version
	// ProtoVersion 5 - sync with no entry space
	// ProtoVersion 6 - sync with key value messages
	// ProtoVersion 7 - sync with new invites
	// ProtoVersion 8 - acl: transfer ownership feature
	CompatibleVersion = uint32(7)
	ProtoVersion      = uint32(8)
)

var (
	compatibleVersions = []uint32{CompatibleVersion, ProtoVersion}
)

func New() SecureService {
	return &secureService{}
}

type SecureService interface {
	SecureOutbound(ctx context.Context, conn net.Conn) (cctx context.Context, err error)
	HandshakeOutbound(ctx context.Context, conn io.ReadWriteCloser, peerId string) (cctx context.Context, err error)
	SecureInbound(ctx context.Context, conn net.Conn) (cctx context.Context, err error)
	HandshakeInbound(ctx context.Context, conn io.ReadWriteCloser, remotePeerId string) (cctx context.Context, err error)
	TlsConfig() (*tls.Config, <-chan crypto.PubKey, error)
	app.Component
}

type secureService struct {
	p2pTr              *libp2ptls.Transport
	account            *accountdata.AccountKeys
	key                crypto.PrivKey
	nodeconf           nodeconf.Service
	protoVersion       uint32
	compatibleVersions []uint32

	noVerifyChecker  handshake.CredentialChecker
	peerSignVerifier handshake.CredentialChecker
	inboundChecker   handshake.CredentialChecker
}

func (s *secureService) Init(a *app.App) (err error) {
	if s.protoVersion == 0 {
		s.protoVersion = ProtoVersion
	}
	if len(s.compatibleVersions) == 0 {
		s.compatibleVersions = compatibleVersions
	}
	account := a.MustComponent(commonaccount.CName).(commonaccount.Service)

	var conf Config
	if cg, ok := a.Component("config").(configGetter); ok {
		conf = cg.GetSecureService()
	}

	peerKey, err := account.Account().PeerKey.Raw()
	if err != nil {
		return
	}
	if s.key, err = crypto.UnmarshalEd25519PrivateKey(peerKey); err != nil {
		return
	}
	s.noVerifyChecker = newNoVerifyChecker(s.protoVersion, s.compatibleVersions, a.VersionName())
	s.peerSignVerifier = newPeerSignVerifier(s.protoVersion, s.compatibleVersions, a.VersionName(), account.Account())

	s.nodeconf = a.MustComponent(nodeconf.CName).(nodeconf.Service)

	s.inboundChecker = s.noVerifyChecker
	confTypes := s.nodeconf.NodeTypes(account.Account().PeerId)
	if conf.RequireClientAuth || len(confTypes) > 0 {
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

func (s *secureService) SecureInbound(ctx context.Context, conn net.Conn) (cctx context.Context, err error) {
	sc, err := s.p2pTr.SecureInbound(ctx, conn, "")
	if err != nil {
		return nil, handshake.HandshakeError{
			Err: err,
		}
	}
	return s.HandshakeInbound(ctx, sc, sc.RemotePeer().String())
}

func (s *secureService) HandshakeInbound(ctx context.Context, conn io.ReadWriteCloser, peerId string) (cctx context.Context, err error) {
	res, err := handshake.IncomingHandshake(ctx, conn, peerId, s.inboundChecker)
	if err != nil {
		return nil, err
	}
	cctx = context.Background()
	cctx = peer.CtxWithPeerId(cctx, peerId)
	cctx = peer.CtxWithIdentity(cctx, res.Identity)
	cctx = peer.CtxWithClientVersion(cctx, res.ClientVersion)
	cctx = peer.CtxWithProtoVersion(cctx, res.ProtoVersion)
	return
}

func (s *secureService) SecureOutbound(ctx context.Context, conn net.Conn) (cctx context.Context, err error) {
	sc, err := s.p2pTr.SecureOutbound(ctx, conn, "")
	if err != nil {
		return nil, handshake.HandshakeError{Err: err}
	}
	return s.HandshakeOutbound(ctx, sc, sc.RemotePeer().String())
}

func (s *secureService) HandshakeOutbound(ctx context.Context, conn io.ReadWriteCloser, peerId string) (cctx context.Context, err error) {
	confTypes := s.nodeconf.NodeTypes(peerId)
	var checker handshake.CredentialChecker
	if CtxIsAccountCheckAllowed(ctx) || len(confTypes) > 0 {
		checker = s.peerSignVerifier
	} else {
		checker = s.noVerifyChecker
	}
	res, err := handshake.OutgoingHandshake(ctx, conn, peerId, checker)
	if err != nil {
		return nil, err
	}
	cctx = context.Background()
	cctx = peer.CtxWithPeerId(cctx, peerId)
	cctx = peer.CtxWithIdentity(cctx, res.Identity)
	cctx = peer.CtxWithClientVersion(cctx, res.ClientVersion)
	cctx = peer.CtxWithProtoVersion(cctx, res.ProtoVersion)
	return cctx, nil
}

func (s *secureService) TlsConfig() (*tls.Config, <-chan crypto.PubKey, error) {
	p2pIdn, err := libp2ptls.NewIdentity(s.key)
	if err != nil {
		return nil, nil, err
	}
	conf, keyCh := p2pIdn.ConfigForPeer("")
	conf.NextProtos = []string{"anysync"}
	return conf, keyCh, nil
}

package secureservice

import (
	"context"
	commonaccount "github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/app/logger"
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
	TLSListener(lis net.Listener, timeoutMillis int) ContextListener
	BasicListener(lis net.Listener, timeoutMillis int) ContextListener
	TLSConn(ctx context.Context, conn net.Conn) (sec.SecureConn, error)
	app.Component
}

type secureService struct {
	key crypto.PrivKey
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

	log.Info("secure service init", zap.String("peerId", account.Account().PeerId))

	return nil
}

func (s *secureService) Name() (name string) {
	return CName
}

func (s *secureService) TLSListener(lis net.Listener, timeoutMillis int) ContextListener {
	return newTLSListener(s.key, lis, timeoutMillis)
}

func (s *secureService) BasicListener(lis net.Listener, timeoutMillis int) ContextListener {
	return newBasicListener(lis, timeoutMillis)
}

func (s *secureService) TLSConn(ctx context.Context, conn net.Conn) (sec.SecureConn, error) {
	tr, err := libp2ptls.New(libp2ptls.ID, s.key, nil)
	if err != nil {
		return nil, err
	}
	return tr.SecureOutbound(ctx, conn, "")
}

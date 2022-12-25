package secureservice

import (
	"context"
	commonaccount "github.com/anytypeio/go-anytype-infrastructure-experiments/common/accountservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/sec"
	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"
	"go.uber.org/zap"
	"net"
)

type HandshakeError error

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
	account := a.MustComponent(config.CName).(commonaccount.ConfigGetter).GetAccount()
	pkb, err := keys.DecodeBytesFromString(account.PeerKey)
	if err != nil {
		return
	}
	if s.key, err = crypto.UnmarshalEd25519PrivateKey(pkb); err != nil {
		return
	}

	log.Info("secure service init", zap.String("peerId", account.PeerId))

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
	tr, err := libp2ptls.New(s.key)
	if err != nil {
		return nil, err
	}
	return tr.SecureOutbound(ctx, conn, "")
}

package secure

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/sec"
	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"
	"go.uber.org/zap"
	"net"
)

type HandshakeError error

const CName = "net/secure"

var log = logger.NewNamed(CName)

func New() Service {
	return &service{}
}

type Service interface {
	TLSListener(lis net.Listener) ContextListener
	TLSConn(ctx context.Context, conn net.Conn) (sec.SecureConn, error)
	app.Component
}

type service struct {
	key crypto.PrivKey
}

func (s *service) Init(a *app.App) (err error) {
	account := a.MustComponent(config.CName).(*config.Config).Account
	pkb, err := keys.DecodeBytesFromString(account.SigningKey)
	if err != nil {
		return
	}
	if s.key, err = crypto.UnmarshalEd25519PrivateKey(pkb); err != nil {
		return
	}

	pid, err := peer.Decode(account.PeerId)
	if err != nil {
		return
	}

	var testData = []byte("test data")
	sign, err := s.key.Sign(testData)
	if err != nil {
		return
	}
	pubKey, err := pid.ExtractPublicKey()
	if err != nil {
		return
	}
	ok, err := pubKey.Verify(testData, sign)
	if err != nil {
		return
	}
	if !ok {
		return fmt.Errorf("peerId and privateKey mismatched")
	}

	log.Info("secure service init", zap.String("peerId", account.PeerId))

	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) TLSListener(lis net.Listener) ContextListener {
	return newTLSListener(s.key, lis)
}

func (s *service) TLSConn(ctx context.Context, conn net.Conn) (sec.SecureConn, error) {
	tr, err := libp2ptls.New(s.key)
	if err != nil {
		return nil, err
	}
	return tr.SecureOutbound(ctx, conn, "")
}

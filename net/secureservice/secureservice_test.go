package secureservice

import (
	"context"
	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/mock_nodeconf"
	"github.com/anyproto/any-sync/testutil/testnodeconf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"net"
	"testing"
)

var ctx = context.Background()

func TestHandshake(t *testing.T) {
	nc := testnodeconf.GenNodeConfig(2)
	fxS := newFixture(t, nc, nc.GetAccountService(0), 1, []uint32{1})
	defer fxS.Finish(t)
	sc, cc := net.Pipe()

	type acceptRes struct {
		ctx  context.Context
		conn net.Conn
		err  error
	}
	resCh := make(chan acceptRes)
	go func() {
		var ar acceptRes
		ar.ctx, ar.err = fxS.SecureInbound(ctx, sc)
		resCh <- ar
	}()

	fxC := newFixture(t, nc, nc.GetAccountService(1), 1, []uint32{1})
	defer fxC.Finish(t)

	const admissionToken = "client-admission-token"
	cctx, err := fxC.SecureOutbound(CtxWithOutboundAdmissionToken(ctx, admissionToken), cc)
	require.NoError(t, err)
	ctxPeerId, err := peer.CtxPeerId(cctx)
	require.NoError(t, err)
	assert.Equal(t, nc.GetAccountService(0).Account().PeerId, ctxPeerId)
	res := <-resCh
	require.NoError(t, res.err)
	peerId, err := peer.CtxPeerId(res.ctx)
	require.NoError(t, err)
	accId, err := peer.CtxIdentity(res.ctx)
	require.NoError(t, err)
	marshalledId, _ := nc.GetAccountService(1).Account().SignKey.GetPublic().Marshall()
	assert.Equal(t, nc.GetAccountService(1).Account().PeerId, peerId)
	assert.Equal(t, marshalledId, accId)
	assert.Equal(t, admissionToken, CtxRemoteAdmissionToken(res.ctx))
	assert.Empty(t, CtxOutboundAdmissionToken(res.ctx))
}

func TestHandshakeAdmissionRequired(t *testing.T) {
	nc := testnodeconf.GenNodeConfig(2)
	verifier := &recordingAdmissionVerifier{
		decision: AdmissionDecision{Allowed: true, Reason: "ok"},
	}
	secureConf := Config{Admission: AdmissionConfig{Enabled: true, Required: true}}
	fxS := newFixtureWithSecureConfig(t, nc, nc.GetAccountService(0), 1, []uint32{1}, &secureConf, verifier)
	defer fxS.Finish(t)
	sc, cc := net.Pipe()

	type acceptRes struct {
		ctx context.Context
		err error
	}
	resCh := make(chan acceptRes)
	go func() {
		var ar acceptRes
		ar.ctx, ar.err = fxS.SecureInbound(ctx, sc)
		resCh <- ar
	}()

	fxC := newFixture(t, nc, nc.GetAccountService(1), 1, []uint32{1})
	defer fxC.Finish(t)

	const admissionToken = "client-admission-token"
	_, err := fxC.SecureOutbound(CtxWithOutboundAdmissionToken(ctx, admissionToken), cc)
	require.NoError(t, err)
	res := <-resCh
	require.NoError(t, res.err)
	assert.Equal(t, admissionToken, CtxRemoteAdmissionToken(res.ctx))

	requests := verifier.Requests()
	require.Len(t, requests, 1)
	assert.Equal(t, admissionToken, requests[0].Token)
	assert.Equal(t, "test-network", requests[0].NetworkID)
	assert.Equal(t, nc.GetAccountService(1).Account().PeerId, requests[0].PeerID)
	assert.NotEmpty(t, requests[0].Identity)
}

func TestHandshakeAdmissionRequiredRejectsMissingToken(t *testing.T) {
	nc := testnodeconf.GenNodeConfig(2)
	verifier := &recordingAdmissionVerifier{
		decision: AdmissionDecision{Allowed: true, Reason: "ok"},
	}
	secureConf := Config{Admission: AdmissionConfig{Enabled: true, Required: true}}
	fxS := newFixtureWithSecureConfig(t, nc, nc.GetAccountService(0), 1, []uint32{1}, &secureConf, verifier)
	defer fxS.Finish(t)
	sc, cc := net.Pipe()

	type acceptRes struct {
		err error
	}
	resCh := make(chan acceptRes)
	go func() {
		var ar acceptRes
		_, ar.err = fxS.SecureInbound(ctx, sc)
		resCh <- ar
	}()

	fxC := newFixture(t, nc, nc.GetAccountService(1), 1, []uint32{1})
	defer fxC.Finish(t)

	_, err := fxC.SecureOutbound(ctx, cc)
	assert.Equal(t, handshake.ErrPeerDeclinedCredentials, err)
	res := <-resCh
	assert.Equal(t, handshake.ErrInvalidCredentials, res.err)
	assert.Empty(t, verifier.Requests())
}

func TestInitAdmissionEnabledRequiresVerifier(t *testing.T) {
	nc := testnodeconf.GenNodeConfig(1)
	secureConf := Config{Admission: AdmissionConfig{Enabled: true}}
	a := new(app.App)
	a.Register(nc.GetAccountService(0)).Register(testSecureConfig{nodeConf: nc, secureConf: secureConf})

	ss := New().(*secureService)
	err := ss.Init(a)
	assert.ErrorIs(t, err, ErrAdmissionInvalidConfig)
}

func TestHandshakeIncompatibleVersion(t *testing.T) {
	nc := testnodeconf.GenNodeConfig(2)
	fxS := newFixture(t, nc, nc.GetAccountService(0), 1, []uint32{0, 1})
	defer fxS.Finish(t)
	sc, cc := net.Pipe()

	type acceptRes struct {
		ctx  context.Context
		conn net.Conn
		err  error
	}
	resCh := make(chan acceptRes)
	go func() {
		var ar acceptRes
		ar.ctx, ar.err = fxS.SecureInbound(ctx, sc)
		resCh <- ar
	}()
	fxC := newFixture(t, nc, nc.GetAccountService(1), 2, []uint32{2, 3})
	defer fxC.Finish(t)
	_, err := fxC.SecureOutbound(ctx, cc)
	require.Equal(t, handshake.ErrIncompatibleVersion, err)
	res := <-resCh
	require.Equal(t, handshake.ErrIncompatibleVersion, res.err)
}

func newFixture(t *testing.T, nc *testnodeconf.Config, acc accountservice.Service, protoVersion uint32, cv []uint32) *fixture {
	return newFixtureWithSecureConfig(t, nc, acc, protoVersion, cv, nil, nil)
}

func newFixtureWithSecureConfig(t *testing.T, nc *testnodeconf.Config, acc accountservice.Service, protoVersion uint32, cv []uint32, secureConf *Config, admissionVerifier AdmissionVerifier) *fixture {
	fx := &fixture{
		ctrl: gomock.NewController(t),
		acc:  acc,
		a:    new(app.App),
	}
	if admissionVerifier != nil {
		fx.secureService = NewWithAdmissionVerifier(admissionVerifier).(*secureService)
	} else {
		fx.secureService = New().(*secureService)
	}
	fx.secureService.protoVersion = protoVersion
	fx.secureService.compatibleVersions = cv
	fx.mockNodeConf = mock_nodeconf.NewMockService(fx.ctrl)
	fx.mockNodeConf.EXPECT().Init(gomock.Any())
	fx.mockNodeConf.EXPECT().Name().Return(nodeconf.CName).AnyTimes()
	fx.mockNodeConf.EXPECT().Run(ctx)
	fx.mockNodeConf.EXPECT().Close(ctx)
	fx.mockNodeConf.EXPECT().NodeTypes(gomock.Any()).Return([]nodeconf.NodeType{nodeconf.NodeTypeTree}).AnyTimes()
	fx.mockNodeConf.EXPECT().Configuration().Return(nodeconf.Configuration{NetworkId: "test-network"}).AnyTimes()
	configComponent := app.Component(nc)
	if secureConf != nil {
		configComponent = testSecureConfig{nodeConf: nc, secureConf: *secureConf}
	}
	fx.a.Register(fx.acc).Register(configComponent).Register(fx.mockNodeConf).Register(fx.secureService)
	require.NoError(t, fx.a.Start(ctx))
	return fx
}

type testSecureConfig struct {
	nodeConf   *testnodeconf.Config
	secureConf Config
}

func (t testSecureConfig) Init(a *app.App) error { return nil }

func (t testSecureConfig) Name() string { return "config" }

func (t testSecureConfig) GetSecureService() Config { return t.secureConf }

func (t testSecureConfig) GetNodeConf() nodeconf.Configuration { return t.nodeConf.GetNodeConf() }

type fixture struct {
	*secureService
	a            *app.App
	acc          accountservice.Service
	ctrl         *gomock.Controller
	mockNodeConf *mock_nodeconf.MockService
}

func (fx *fixture) Finish(t *testing.T) {
	require.NoError(t, fx.a.Close(ctx))
}

//go:build !js

package webtransport

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	netpeer "github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/transport"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/mock_nodeconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/any-sync/testutil/testnodeconf"
)

var testCtx = context.Background()

func TestWtTransport_Dial(t *testing.T) {
	fxS := newNativeFixture(t)
	defer fxS.finish(t)
	fxC := newNativeFixture(t)
	defer fxC.finish(t)

	serverPeerId := fxS.acc.Account().PeerId
	dialCtx := netpeer.CtxWithExpectedPeerId(testCtx, serverPeerId)

	mcC, err := fxC.Dial(dialCtx, fxS.addr)
	require.NoError(t, err)

	var mcS transport.MultiConn
	select {
	case mcS = <-fxS.accepter.mcs:
	case <-time.After(time.Second * 10):
		require.Fail(t, "timeout waiting for server accept")
	}

	// Verify peerId in server's connection context
	connPeerId, err := netpeer.CtxPeerId(mcS.Context())
	require.NoError(t, err)
	assert.Equal(t, fxC.acc.Account().PeerId, connPeerId)

	// Exchange data over sub-connections
	var (
		sData     string
		acceptErr error
		copyErr   error
		done      = make(chan struct{})
	)

	go func() {
		defer close(done)
		conn, serr := mcS.Accept()
		if serr != nil {
			acceptErr = serr
			return
		}
		buf := bytes.NewBuffer(nil)
		_, copyErr = io.Copy(buf, conn)
		sData = buf.String()
	}()

	conn, err := mcC.Open(testCtx)
	require.NoError(t, err)
	data := "hello webtransport"
	_, err = conn.Write([]byte(data))
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	<-done

	assert.NoError(t, acceptErr)
	assert.Equal(t, data, sData)
	assert.NoError(t, copyErr)
}

func TestWtTransport_MultipleStreams(t *testing.T) {
	fxS := newNativeFixture(t)
	defer fxS.finish(t)
	fxC := newNativeFixture(t)
	defer fxC.finish(t)

	serverPeerId := fxS.acc.Account().PeerId
	dialCtx := netpeer.CtxWithExpectedPeerId(testCtx, serverPeerId)

	mcC, err := fxC.Dial(dialCtx, fxS.addr)
	require.NoError(t, err)

	var mcS transport.MultiConn
	select {
	case mcS = <-fxS.accepter.mcs:
	case <-time.After(time.Second * 10):
		require.Fail(t, "timeout")
	}

	numStreams := 5
	var wg sync.WaitGroup
	var verified atomic.Int32

	// Server side: accept and echo
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numStreams; i++ {
			conn, err := mcS.Accept()
			if err != nil {
				t.Errorf("server accept error: %v", err)
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c)
			}(conn)
		}
	}()

	// Client side: open streams and exchange data
	for i := 0; i < numStreams; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conn, err := mcC.Open(testCtx)
			if err != nil {
				t.Errorf("client open error: %v", err)
				return
			}
			defer conn.Close()

			msg := fmt.Sprintf("stream-%d", idx)
			_, err = conn.Write([]byte(msg))
			if err != nil {
				t.Errorf("client write error: %v", err)
				return
			}

			buf := make([]byte, 64)
			n, err := conn.Read(buf)
			if err != nil {
				t.Errorf("client read error: %v", err)
				return
			}
			assert.Equal(t, msg, string(buf[:n]))
			verified.Add(1)
		}(i)
	}

	wg.Wait()
	assert.Equal(t, int32(numStreams), verified.Load(), "all streams should have verified data")
}

func TestWtTransport_CertHotReload(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := tmpDir + "/cert.pem"
	keyFile := tmpDir + "/key.pem"

	// Generate initial cert
	writeTestCert(t, certFile, keyFile)

	fxS := newNativeFixtureWithCertFiles(t, certFile, keyFile)
	defer fxS.finish(t)
	fxC := newNativeFixture(t)
	defer fxC.finish(t)

	serverPeerId := fxS.acc.Account().PeerId
	dialCtx := netpeer.CtxWithExpectedPeerId(testCtx, serverPeerId)

	// First connection with original cert
	mc1, err := fxC.Dial(dialCtx, fxS.addr)
	require.NoError(t, err)
	select {
	case <-fxS.accepter.mcs:
	case <-time.After(time.Second * 10):
		require.Fail(t, "timeout")
	}
	_ = mc1.Close()

	// Replace cert files with new cert
	writeTestCert(t, certFile, keyFile)

	// Second connection should use new cert (GetCertificate re-reads from disk)
	mc2, err := fxC.Dial(dialCtx, fxS.addr)
	require.NoError(t, err)
	select {
	case <-fxS.accepter.mcs:
	case <-time.After(time.Second * 10):
		require.Fail(t, "timeout")
	}
	_ = mc2.Close()
}

func TestWtTransport_CORS(t *testing.T) {
	fxS := newNativeFixture(t)
	defer fxS.finish(t)

	// We can't make a regular HTTP request to an HTTP/3 server easily,
	// but we can verify the CORS headers are set by checking that
	// the handleUpgrade function sets them. We test this indirectly
	// via a successful Dial (the upgrade handler sets CORS headers).
	// Let's verify the handler logic directly.
	rec := &headerRecorder{headers: make(http.Header)}
	req, err := http.NewRequest(http.MethodOptions, "/webtransport", nil)
	require.NoError(t, err)

	fxS.handleUpgrade(rec, req)

	assert.Equal(t, "*", rec.headers.Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rec.headers.Get("Access-Control-Allow-Methods"), "GET")
	assert.Contains(t, rec.headers.Get("Access-Control-Allow-Methods"), "OPTIONS")
	assert.Equal(t, "*", rec.headers.Get("Access-Control-Allow-Headers"))
	assert.Equal(t, http.StatusOK, rec.statusCode)
}

func TestWtTransport_Close(t *testing.T) {
	fxS := newNativeFixture(t)
	defer fxS.finish(t)
	fxC := newNativeFixture(t)
	defer fxC.finish(t)

	serverPeerId := fxS.acc.Account().PeerId
	dialCtx := netpeer.CtxWithExpectedPeerId(testCtx, serverPeerId)

	mcC, err := fxC.Dial(dialCtx, fxS.addr)
	require.NoError(t, err)

	select {
	case <-fxS.accepter.mcs:
	case <-time.After(time.Second * 10):
		require.Fail(t, "timeout")
	}

	assert.False(t, mcC.IsClosed())
	err = mcC.Close()
	assert.NoError(t, err)

	select {
	case <-mcC.CloseChan():
	case <-time.After(time.Second * 5):
		require.Fail(t, "CloseChan not signaled")
	}
	assert.True(t, mcC.IsClosed())
}

func TestWtTransport_Addr(t *testing.T) {
	fxS := newNativeFixture(t)
	defer fxS.finish(t)
	fxC := newNativeFixture(t)
	defer fxC.finish(t)

	serverPeerId := fxS.acc.Account().PeerId
	dialCtx := netpeer.CtxWithExpectedPeerId(testCtx, serverPeerId)

	mcC, err := fxC.Dial(dialCtx, fxS.addr)
	require.NoError(t, err)

	assert.Contains(t, mcC.Addr(), "webtransport://")

	var mcS transport.MultiConn
	select {
	case mcS = <-fxS.accepter.mcs:
	case <-time.After(time.Second * 10):
		require.Fail(t, "timeout")
	}
	assert.Contains(t, mcS.Addr(), "webtransport://")
}

func TestWtTransport_HandshakePeerIdVerification(t *testing.T) {
	fxS := newNativeFixture(t)
	defer fxS.finish(t)
	fxC := newNativeFixture(t)
	defer fxC.finish(t)

	// Verify that the server sees the client's peerId
	serverPeerId := fxS.acc.Account().PeerId
	clientPeerId := fxC.acc.Account().PeerId
	dialCtx := netpeer.CtxWithExpectedPeerId(testCtx, serverPeerId)

	_, err := fxC.Dial(dialCtx, fxS.addr)
	require.NoError(t, err)

	var mcS transport.MultiConn
	select {
	case mcS = <-fxS.accepter.mcs:
	case <-time.After(time.Second * 10):
		require.Fail(t, "timeout")
	}

	connPeerId, err := netpeer.CtxPeerId(mcS.Context())
	require.NoError(t, err)
	assert.Equal(t, clientPeerId, connPeerId)
}

func TestWtTransport_HandshakeMismatchedPeerIdRejected(t *testing.T) {
	fxS := newNativeFixture(t)
	defer fxS.finish(t)
	fxC := newNativeFixture(t)
	defer fxC.finish(t)

	// Use a wrong expected peerId — the handshake should fail
	wrongPeerId := "12D3KooWWrongPeerIdThatDoesNotMatchAnything"
	dialCtx := netpeer.CtxWithExpectedPeerId(testCtx, wrongPeerId)

	_, err := fxC.Dial(dialCtx, fxS.addr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "handshake")

	// Server should not have accepted any connection
	select {
	case <-fxS.accepter.mcs:
		t.Fatal("server should not have accepted a connection with mismatched peerId")
	case <-time.After(500 * time.Millisecond):
		// Expected: no accept
	}
}

func TestWtTransport_ShutdownClosesListeners(t *testing.T) {
	fx := newNativeFixture(t)

	// Verify we have UDP connections
	assert.NotEmpty(t, fx.udpConns)

	// Close the fixture (calls Close)
	fx.finish(t)

	// After close, the listener context should be cancelled
	select {
	case <-fx.listCtx.Done():
	default:
		t.Fatal("listCtx should be cancelled after Close")
	}
}

// --- Test helpers ---

// headerRecorder is a minimal http.ResponseWriter for testing CORS headers.
type headerRecorder struct {
	headers    http.Header
	statusCode int
}

func (r *headerRecorder) Header() http.Header        { return r.headers }
func (r *headerRecorder) Write(b []byte) (int, error) { return len(b), nil }
func (r *headerRecorder) WriteHeader(code int)        { r.statusCode = code }

type nativeFixture struct {
	*wtTransport
	a            *app.App
	ctrl         *gomock.Controller
	mockNodeConf *mock_nodeconf.MockService
	acc          *accounttest.AccountTestService
	accepter     *nativeTestAccepter
	addr         string
}

func newNativeFixture(t *testing.T) *nativeFixture {
	tmpDir := t.TempDir()
	certFile := tmpDir + "/cert.pem"
	keyFile := tmpDir + "/key.pem"
	writeTestCert(t, certFile, keyFile)
	return newNativeFixtureWithCertFiles(t, certFile, keyFile)
}

func newNativeFixtureWithCertFiles(t *testing.T, certFile, keyFile string) *nativeFixture {
	fx := &nativeFixture{
		wtTransport: New().(*wtTransport),
		ctrl:        gomock.NewController(t),
		acc:         &accounttest.AccountTestService{},
		accepter:    &nativeTestAccepter{mcs: make(chan transport.MultiConn, 100)},
		a:           new(app.App),
	}

	fx.mockNodeConf = mock_nodeconf.NewMockService(fx.ctrl)
	fx.mockNodeConf.EXPECT().Init(gomock.Any())
	fx.mockNodeConf.EXPECT().Name().Return(nodeconf.CName).AnyTimes()
	fx.mockNodeConf.EXPECT().Run(testCtx)
	fx.mockNodeConf.EXPECT().Close(testCtx)
	fx.mockNodeConf.EXPECT().NodeTypes(gomock.Any()).Return([]nodeconf.NodeType{nodeconf.NodeTypeTree}).AnyTimes()

	conf := &nativeTestConf{
		Config:   testnodeconf.GenNodeConfig(1),
		certFile: certFile,
		keyFile:  keyFile,
	}

	fx.a.Register(fx.acc).
		Register(conf).
		Register(fx.mockNodeConf).
		Register(secureservice.New()).
		Register(fx.wtTransport).
		Register(fx.accepter)
	require.NoError(t, fx.a.Start(testCtx))

	// Extract the listen address
	if len(fx.udpConns) > 0 {
		fx.addr = fx.udpConns[0].LocalAddr().String()
	}
	return fx
}

func (fx *nativeFixture) finish(t *testing.T) {
	require.NoError(t, fx.a.Close(testCtx))
	fx.ctrl.Finish()
}

type nativeTestConf struct {
	*testnodeconf.Config
	certFile string
	keyFile  string
}

func (c *nativeTestConf) GetWebTransport() Config {
	return Config{
		ListenAddrs:     []string{"127.0.0.1:0"},
		Path:            "/webtransport",
		CertFile:        c.certFile,
		KeyFile:         c.keyFile,
		WriteTimeoutSec: 10,
		DialTimeoutSec:  30,
	}
}

type nativeTestAccepter struct {
	err error
	mcs chan transport.MultiConn
}

func (a *nativeTestAccepter) Accept(mc transport.MultiConn) error {
	a.mcs <- mc
	return a.err
}

func (a *nativeTestAccepter) Init(app *app.App) error {
	app.MustComponent(CName).(transport.Transport).SetAccepter(a)
	return nil
}

func (a *nativeTestAccepter) Name() string { return "testAccepter" }

// writeTestCert generates a self-signed certificate and writes PEM files.
func writeTestCert(t *testing.T, certFile, keyFile string) {
	t.Helper()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "test"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:         true,
		BasicConstraintsValid: true,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &caKey.PublicKey, caKey)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(caKey)})

	require.NoError(t, writeFile(certFile, certPEM))
	require.NoError(t, writeFile(keyFile, keyPEM))
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

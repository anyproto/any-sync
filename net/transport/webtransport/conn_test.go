//go:build !js

package webtransport

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	wt "github.com/quic-go/webtransport-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/transport"
)

// testSessionPair creates a client and server WebTransport session pair for testing.
func testSessionPair(t *testing.T) (clientSess, serverSess *wt.Session, closeFn func()) {
	t.Helper()

	tlsConf, certPool := generateTestTLS(t)

	serverSessCh := make(chan *wt.Session, 1)
	s := &wt.Server{
		H3: &http3.Server{
			TLSConfig: tlsConf,
			QUICConfig: &quic.Config{
				EnableDatagrams:                  true,
				EnableStreamResetPartialDelivery: true,
			},
		},
		CheckOrigin: func(*http.Request) bool { return true },
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/wt", func(w http.ResponseWriter, r *http.Request) {
		sess, err := s.Upgrade(w, r)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		serverSessCh <- sess
	})
	s.H3.Handler = mux

	laddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	require.NoError(t, err)
	udpConn, err := net.ListenUDP("udp", laddr)
	require.NoError(t, err)

	wt.ConfigureHTTP3Server(s.H3)
	go s.Serve(udpConn)

	d := wt.Dialer{
		TLSClientConfig: &tls.Config{RootCAs: certPool},
		QUICConfig: &quic.Config{
			EnableDatagrams:                  true,
			EnableStreamResetPartialDelivery: true,
		},
	}

	url := fmt.Sprintf("https://localhost:%d/wt", udpConn.LocalAddr().(*net.UDPAddr).Port)
	rsp, cSess, err := d.Dial(context.Background(), url, nil)
	require.NoError(t, err)
	require.Equal(t, 200, rsp.StatusCode)

	sSess := <-serverSessCh

	return cSess, sSess, func() {
		cSess.CloseWithError(0, "")
		sSess.CloseWithError(0, "")
		s.Close()
		d.Close()
		udpConn.Close()
	}
}

func generateTestTLS(t *testing.T) (*tls.Config, *x509.CertPool) {
	t.Helper()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	ca, err := x509.ParseCertificate(caBytes)
	require.NoError(t, err)

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		DNSNames:     []string{"localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafBytes, err := x509.CreateCertificate(rand.Reader, leafTemplate, ca, &leafKey.PublicKey, caKey)
	require.NoError(t, err)

	certPool := x509.NewCertPool()
	certPool.AddCert(ca)

	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{leafBytes},
			PrivateKey:  leafKey,
		}},
		NextProtos: []string{http3.NextProtoH3},
	}

	return tlsConf, certPool
}

func TestWtAddr(t *testing.T) {
	a := wtAddr{addr: "192.168.1.1:443"}
	assert.Equal(t, "webtransport", a.Network())
	assert.Equal(t, "192.168.1.1:443", a.String())
}

func TestWtNetConn_ReadWriteClose(t *testing.T) {
	clientSess, serverSess, closeFn := testSessionPair(t)
	defer closeFn()

	// Open stream and write data (writing sends the WT frame header,
	// which is needed for the server to route the stream)
	clientStream, err := clientSess.OpenStreamSync(context.Background())
	require.NoError(t, err)

	data := []byte("hello webtransport")
	localAddr := wtAddr{addr: "local"}
	remoteAddr := wtAddr{addr: "remote"}

	clientConn := wtNetConn{
		Stream:     clientStream,
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
	}

	n, err := clientConn.Write(data)
	require.NoError(t, err)
	assert.Equal(t, len(data), n)

	// Now server can accept the stream
	serverStream, err := serverSess.AcceptStream(context.Background())
	require.NoError(t, err)

	serverConn := wtNetConn{
		Stream:     serverStream,
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
	}

	buf := make([]byte, 256)
	n, err = serverConn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, data, buf[:n])

	// Test LocalAddr and RemoteAddr
	assert.Equal(t, localAddr, clientConn.LocalAddr())
	assert.Equal(t, remoteAddr, clientConn.RemoteAddr())

	// Test Close
	require.NoError(t, clientConn.Close())
	require.NoError(t, serverConn.Close())
}

func TestWtNetConn_WriteWithTimeout(t *testing.T) {
	clientSess, serverSess, closeFn := testSessionPair(t)
	defer closeFn()

	clientStream, err := clientSess.OpenStreamSync(context.Background())
	require.NoError(t, err)

	clientConn := wtNetConn{
		Stream:       clientStream,
		writeTimeout: 5 * time.Second,
		localAddr:    wtAddr{addr: "local"},
		remoteAddr:   wtAddr{addr: "remote"},
	}

	data := []byte("timeout test data")
	n, err := clientConn.Write(data)
	require.NoError(t, err)
	assert.Equal(t, len(data), n)

	// Server accepts after data is written
	serverStream, err := serverSess.AcceptStream(context.Background())
	require.NoError(t, err)

	buf := make([]byte, 256)
	n, err = serverStream.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, data, buf[:n])
}

func TestWtMultiConn_OpenAccept(t *testing.T) {
	clientSess, serverSess, closeFn := testSessionPair(t)
	defer closeFn()

	ctx := context.Background()
	mc := newConn(ctx, clientSess, "localhost:1234", 0, 5*time.Second)
	mcServer := newConn(ctx, serverSess, "localhost:5678", 0, 5*time.Second)

	// Client opens a stream and writes data
	conn, err := mc.Open(ctx)
	require.NoError(t, err)

	data := "hello from multiconn"
	_, err = conn.Write([]byte(data))
	require.NoError(t, err)

	// Server accepts the stream and reads data
	sConn, err := mcServer.Accept()
	require.NoError(t, err)

	buf := make([]byte, 256)
	n, err := sConn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, data, string(buf[:n]))

	require.NoError(t, conn.Close())
}

func TestWtMultiConn_Addr(t *testing.T) {
	clientSess, _, closeFn := testSessionPair(t)
	defer closeFn()

	mc := newConn(context.Background(), clientSess, "example.com:443", 0, 5*time.Second)
	assert.Equal(t, "webtransport://example.com:443", mc.Addr())
}

func TestWtMultiConn_Context(t *testing.T) {
	clientSess, _, closeFn := testSessionPair(t)
	defer closeFn()

	mc := newConn(context.Background(), clientSess, "example.com:443", 0, 5*time.Second)
	addr := peer.CtxPeerAddr(mc.Context())
	assert.Equal(t, "webtransport://example.com:443", addr)
}

func TestWtMultiConn_IsClosed(t *testing.T) {
	clientSess, _, closeFn := testSessionPair(t)
	defer closeFn()

	mc := newConn(context.Background(), clientSess, "localhost:1234", 0, 5*time.Second)
	assert.False(t, mc.IsClosed())

	require.NoError(t, mc.Close())

	select {
	case <-mc.CloseChan():
	case <-time.After(5 * time.Second):
		t.Fatal("CloseChan should be closed after Close()")
	}
	assert.True(t, mc.IsClosed())
}

func TestWtMultiConn_CloseChan(t *testing.T) {
	clientSess, _, closeFn := testSessionPair(t)
	defer closeFn()

	mc := newConn(context.Background(), clientSess, "localhost:1234", 0, 5*time.Second)

	select {
	case <-mc.CloseChan():
		t.Fatal("CloseChan should not be closed yet")
	default:
	}

	require.NoError(t, mc.Close())

	select {
	case <-mc.CloseChan():
	case <-time.After(5 * time.Second):
		t.Fatal("CloseChan should be closed after Close()")
	}
}

func TestWtMultiConn_Close(t *testing.T) {
	clientSess, _, closeFn := testSessionPair(t)
	defer closeFn()

	mc := newConn(context.Background(), clientSess, "localhost:1234", 0, 5*time.Second)
	err := mc.Close()
	assert.NoError(t, err)
}

func TestWtMultiConn_InterfaceCompliance(t *testing.T) {
	var _ transport.MultiConn = (*wtMultiConn)(nil)
}

package quic

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/transport"
	"github.com/anyproto/any-sync/net/transport/quic/mock_quic"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/mock_nodeconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/any-sync/testutil/testnodeconf"
)

var ctx = context.Background()

func TestQuicTransport_Dial(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	mcC, err := fxC.Dial(ctx, fxS.addr)
	require.NoError(t, err)
	var mcS transport.MultiConn
	select {
	case mcS = <-fxS.accepter.mcs:
	case <-time.After(time.Second * 5):
		require.True(t, false, "timeout")
	}

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
		return
	}()

	conn, err := mcC.Open(ctx)
	require.NoError(t, err)
	data := "some data"
	_, err = conn.Write([]byte(data))
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	<-done

	assert.NoError(t, acceptErr)
	assert.Equal(t, data, sData)
	assert.NoError(t, copyErr)
}

// no deadline - 69100 rps
// common write deadline - 66700 rps
// subconn write deadline - 67100 rps
func TestWriteBenchReuse(t *testing.T) {
	t.Skip()
	var (
		numSubConn = 10
		numWrites  = 10000
	)

	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	mcC, err := fxC.Dial(ctx, fxS.addr)
	require.NoError(t, err)
	mcS := <-fxS.accepter.mcs

	go func() {
		for i := 0; i < numSubConn; i++ {
			conn, err := mcS.Accept()
			require.NoError(t, err)
			go func(sc net.Conn) {
				var b = make([]byte, 1024)
				for {
					n, _ := sc.Read(b)
					if n > 0 {
						sc.Write(b[:n])
					} else {
						break
					}
				}
			}(conn)
		}
	}()

	var wg sync.WaitGroup
	wg.Add(numSubConn)
	st := time.Now()
	for i := 0; i < numSubConn; i++ {
		conn, err := mcC.Open(ctx)
		require.NoError(t, err)
		go func(sc net.Conn) {
			defer sc.Close()
			defer wg.Done()
			for j := 0; j < numWrites; j++ {
				var b = []byte("some data some data some data some data some data some data some data some data some data")
				sc.Write(b)
				sc.Read(b)
			}
		}(conn)
	}
	wg.Wait()
	dur := time.Since(st)
	t.Logf("%.2f req per sec", float64(numWrites*numSubConn)/dur.Seconds())
}

func TestWriteBenchNew(t *testing.T) {
	t.Skip()
	var (
		numSubConn = 10
		numWrites  = 10000
	)

	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	mcC, err := fxC.Dial(ctx, fxS.addr)
	require.NoError(t, err)
	mcS := <-fxS.accepter.mcs

	go func() {
		for i := 0; i < numSubConn; i++ {
			require.NoError(t, err)
			go func() {
				var b = make([]byte, 1024)
				for {
					conn, _ := mcS.Accept()
					n, _ := conn.Read(b)
					if n > 0 {
						conn.Write(b[:n])
					} else {
						_ = conn.Close()
						break
					}
					conn.Close()
				}
			}()
		}
	}()

	var wg sync.WaitGroup
	wg.Add(numSubConn)
	st := time.Now()
	for i := 0; i < numSubConn; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numWrites; j++ {
				sc, err := mcC.Open(ctx)
				require.NoError(t, err)
				var b = []byte("some data some data some data some data some data some data some data some data some data")
				sc.Write(b)
				sc.Read(b)
				sc.Close()
			}
		}()
	}
	wg.Wait()
	dur := time.Since(st)
	t.Logf("%.2f req per sec", float64(numWrites*numSubConn)/dur.Seconds())
}

type fixture struct {
	*quicTransport
	a            *app.App
	ctrl         *gomock.Controller
	mockNodeConf *mock_nodeconf.MockService
	acc          *accounttest.AccountTestService
	accepter     *testAccepter
	addr         string
}

func newFixture(t *testing.T) *fixture {
	fx := &fixture{
		quicTransport: New().(*quicTransport),
		ctrl:          gomock.NewController(t),
		acc:           &accounttest.AccountTestService{},
		accepter:      &testAccepter{mcs: make(chan transport.MultiConn, 100)},
		a:             new(app.App),
	}

	fx.mockNodeConf = mock_nodeconf.NewMockService(fx.ctrl)
	fx.mockNodeConf.EXPECT().Init(gomock.Any())
	fx.mockNodeConf.EXPECT().Name().Return(nodeconf.CName).AnyTimes()
	fx.mockNodeConf.EXPECT().Run(ctx)
	fx.mockNodeConf.EXPECT().Close(ctx)
	fx.mockNodeConf.EXPECT().NodeTypes(gomock.Any()).Return([]nodeconf.NodeType{nodeconf.NodeTypeTree}).AnyTimes()
	fx.a.Register(fx.acc).Register(newTestConf()).Register(fx.mockNodeConf).Register(secureservice.New()).Register(fx.quicTransport).Register(fx.accepter)
	require.NoError(t, fx.a.Start(ctx))
	fx.addr = fx.listeners[0].Addr().String()
	return fx
}

func (fx *fixture) finish(t *testing.T) {
	require.NoError(t, fx.a.Close(ctx))
	fx.ctrl.Finish()
}

func newTestConf() *testConf {
	return &testConf{testnodeconf.GenNodeConfig(1)}
}

type testConf struct {
	*testnodeconf.Config
}

func (c *testConf) GetQuic() Config {
	return Config{
		ListenAddrs:     []string{"127.0.0.1:0"},
		WriteTimeoutSec: 10,
		DialTimeoutSec:  10,
	}
}

type testAccepter struct {
	err error
	mcs chan transport.MultiConn
}

func (t *testAccepter) Accept(mc transport.MultiConn) (err error) {
	t.mcs <- mc
	return t.err
}

func (t *testAccepter) Init(a *app.App) (err error) {
	a.MustComponent(CName).(transport.Transport).SetAccepter(t)
	return nil
}

func (t *testAccepter) Name() (name string) { return "testAccepter" }

func TestQuicMultiConn_Close(t *testing.T) {
	t.Run("close with udp connection", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockConn := mock_quic.NewMockconnection(ctrl)
		mockUdpConn := mock_quic.NewMockPacketConn(ctrl)
		ctx := context.Background()

		q := &quicMultiConn{
			cctx:         ctx,
			udpConn:      mockUdpConn,
			connection:   mockConn,
			writeTimeout: time.Second,
			closeTimeout: 100 * time.Millisecond,
		}

		done := make(chan struct{})
		mockConn.EXPECT().CloseWithError(quic.ApplicationErrorCode(2), "").DoAndReturn(func(quic.ApplicationErrorCode, string) error {
			close(done)
			return nil
		})
		mockUdpConn.EXPECT().Close().Return(nil)

		err := q.Close()
		assert.NoError(t, err)
		<-done
	})

	t.Run("close with udp connection and quic error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockConn := mock_quic.NewMockconnection(ctrl)
		mockUdpConn := mock_quic.NewMockPacketConn(ctrl)
		ctx := context.Background()

		q := &quicMultiConn{
			cctx:         ctx,
			udpConn:      mockUdpConn,
			connection:   mockConn,
			writeTimeout: time.Second,
			closeTimeout: 100 * time.Millisecond,
		}

		done := make(chan struct{})
		mockConn.EXPECT().CloseWithError(quic.ApplicationErrorCode(2), "").DoAndReturn(func(quic.ApplicationErrorCode, string) error {
			return errors.New("quic error")
		})
		mockUdpConn.EXPECT().Close().DoAndReturn(func() error {
			close(done)
			return nil
		})

		err := q.Close()
		assert.NoError(t, err)
		<-done
	})

	t.Run("close with udp connection and udp error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockConn := mock_quic.NewMockconnection(ctrl)
		mockUdpConn := mock_quic.NewMockPacketConn(ctrl)
		ctx := context.Background()

		q := &quicMultiConn{
			cctx:         ctx,
			udpConn:      mockUdpConn,
			connection:   mockConn,
			writeTimeout: time.Second,
			closeTimeout: 100 * time.Millisecond,
		}

		done := make(chan struct{})
		mockConn.EXPECT().CloseWithError(quic.ApplicationErrorCode(2), "").DoAndReturn(func(quic.ApplicationErrorCode, string) error {
			close(done)
			return nil
		})
		mockUdpConn.EXPECT().Close().Return(errors.New("udp error"))

		err := q.Close()
		assert.NoError(t, err)
		<-done
	})

	t.Run("close without udp connection", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockConn := mock_quic.NewMockconnection(ctrl)
		ctx := context.Background()

		q := &quicMultiConn{
			cctx:         ctx,
			udpConn:      nil,
			connection:   mockConn,
			writeTimeout: time.Second,
			closeTimeout: 100 * time.Millisecond,
		}

		done := make(chan struct{})
		mockConn.EXPECT().CloseWithError(quic.ApplicationErrorCode(2), "").DoAndReturn(func(quic.ApplicationErrorCode, string) error {
			close(done)
			return nil
		})

		err := q.Close()
		assert.NoError(t, err)
		<-done
	})

	t.Run("close timeout triggers udp close", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockConn := mock_quic.NewMockconnection(ctrl)
		mockUdpConn := mock_quic.NewMockPacketConn(ctrl)
		ctx := context.Background()

		q := &quicMultiConn{
			cctx:         ctx,
			udpConn:      mockUdpConn,
			connection:   mockConn,
			writeTimeout: time.Second,
			closeTimeout: 10 * time.Millisecond,
		}

		timeoutDone := make(chan struct{})
		closeDone := make(chan struct{})

		mockUdpConn.EXPECT().Close().DoAndReturn(func() error {
			close(timeoutDone)
			return nil
		})
		mockConn.EXPECT().CloseWithError(quic.ApplicationErrorCode(2), "").DoAndReturn(func(quic.ApplicationErrorCode, string) error {
			<-timeoutDone
			close(closeDone)
			return nil
		})

		err := q.Close()
		assert.NoError(t, err)
		<-closeDone
	})

	t.Run("close timeout without udp connection", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockConn := mock_quic.NewMockconnection(ctrl)
		ctx := context.Background()

		q := &quicMultiConn{
			cctx:         ctx,
			udpConn:      nil,
			connection:   mockConn,
			writeTimeout: time.Second,
			closeTimeout: 10 * time.Millisecond,
		}

		done := make(chan struct{})
		mockConn.EXPECT().CloseWithError(quic.ApplicationErrorCode(2), "").DoAndReturn(func(quic.ApplicationErrorCode, string) error {
			close(done)
			return nil
		})

		err := q.Close()
		assert.NoError(t, err)
		<-done
	})
}

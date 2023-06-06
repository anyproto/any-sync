package yamux

import (
	"bytes"
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/transport"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/mock_nodeconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/any-sync/testutil/testnodeconf"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

var ctx = context.Background()

func TestYamuxTransport_Dial(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	mcC, err := fxC.Dial(ctx, fxS.addr)
	require.NoError(t, err)
	require.Len(t, fxS.accepter.mcs, 1)
	mcS := fxS.accepter.mcs[0]

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
func TestWriteBench(t *testing.T) {
	t.Skip()
	var (
		numSubConn = 10
		numWrites  = 100000
	)

	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	mcC, err := fxC.Dial(ctx, fxS.addr)
	require.NoError(t, err)
	mcS := fxS.accepter.mcs[0]

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

type fixture struct {
	*yamuxTransport
	a            *app.App
	ctrl         *gomock.Controller
	mockNodeConf *mock_nodeconf.MockService
	acc          *accounttest.AccountTestService
	accepter     *testAccepter
	addr         string
}

func newFixture(t *testing.T) *fixture {
	fx := &fixture{
		yamuxTransport: New().(*yamuxTransport),
		ctrl:           gomock.NewController(t),
		acc:            &accounttest.AccountTestService{},
		accepter:       &testAccepter{},
		a:              new(app.App),
	}

	fx.mockNodeConf = mock_nodeconf.NewMockService(fx.ctrl)
	fx.mockNodeConf.EXPECT().Init(gomock.Any())
	fx.mockNodeConf.EXPECT().Name().Return(nodeconf.CName).AnyTimes()
	fx.mockNodeConf.EXPECT().Run(ctx)
	fx.mockNodeConf.EXPECT().Close(ctx)
	fx.mockNodeConf.EXPECT().NodeTypes(gomock.Any()).Return([]nodeconf.NodeType{nodeconf.NodeTypeTree}).AnyTimes()
	fx.a.Register(fx.acc).Register(newTestConf()).Register(fx.mockNodeConf).Register(secureservice.New()).Register(fx.yamuxTransport).Register(fx.accepter)
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

func (c *testConf) GetYamux() Config {
	return Config{
		ListenAddrs:     []string{"127.0.0.1:0"},
		WriteTimeoutSec: 10,
		DialTimeoutSec:  10,
		MaxStreams:      1024,
	}
}

type testAccepter struct {
	err error
	mcs []transport.MultiConn
}

func (t *testAccepter) Accept(mc transport.MultiConn) (err error) {
	t.mcs = append(t.mcs, mc)
	return t.err
}

func (t *testAccepter) Init(a *app.App) (err error) {
	a.MustComponent(CName).(transport.Transport).SetAccepter(t)
	return nil
}

func (t *testAccepter) Name() (name string) { return "testAccepter" }

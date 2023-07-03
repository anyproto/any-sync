package consensusclient

import (
	"context"
	"fmt"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/consensus/consensusproto/consensuserr"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/mock_nodeconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/any-sync/util/cidutil"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func TestService_Watch(t *testing.T) {
	t.Run("not found error", func(t *testing.T) {
		fx := newFixture(t).run(t)
		defer fx.Finish()
		var logId = "1"
		w := &testWatcher{ready: make(chan struct{})}
		require.NoError(t, fx.Watch(logId, w))
		st := fx.testServer.waitStream(t)
		req, err := st.Recv()
		require.NoError(t, err)
		assert.Equal(t, []string{logId}, req.WatchIds)
		require.NoError(t, st.Send(&consensusproto.LogWatchEvent{
			LogId: logId,
			Error: &consensusproto.Err{
				Error: consensusproto.ErrCodes_ErrorOffset + consensusproto.ErrCodes_LogNotFound,
			},
		}))
		<-w.ready
		assert.Equal(t, consensuserr.ErrLogNotFound, w.err)
		fx.testServer.releaseStream <- nil
	})
	t.Run("watcherExists error", func(t *testing.T) {
		fx := newFixture(t).run(t)
		defer fx.Finish()
		var logId = "1"
		w := &testWatcher{}
		require.NoError(t, fx.Watch(logId, w))
		require.Error(t, fx.Watch(logId, w))
		st := fx.testServer.waitStream(t)
		st.Recv()
		fx.testServer.releaseStream <- nil
	})
	t.Run("watch", func(t *testing.T) {
		fx := newFixture(t).run(t)
		defer fx.Finish()
		var logId1 = "1"
		w := &testWatcher{}
		require.NoError(t, fx.Watch(logId1, w))
		st := fx.testServer.waitStream(t)
		req, err := st.Recv()
		require.NoError(t, err)
		assert.Equal(t, []string{logId1}, req.WatchIds)

		var logId2 = "2"
		w = &testWatcher{}
		require.NoError(t, fx.Watch(logId2, w))
		req, err = st.Recv()
		require.NoError(t, err)
		assert.Equal(t, []string{logId2}, req.WatchIds)

		fx.testServer.releaseStream <- nil
	})
}

func TestService_UnWatch(t *testing.T) {
	t.Run("no watcher", func(t *testing.T) {
		fx := newFixture(t).run(t)
		defer fx.Finish()
		require.Error(t, fx.UnWatch("1"))
	})
	t.Run("success", func(t *testing.T) {
		fx := newFixture(t).run(t)
		defer fx.Finish()
		w := &testWatcher{}
		require.NoError(t, fx.Watch("1", w))
		assert.NoError(t, fx.UnWatch("1"))
	})
}

func TestService_Init(t *testing.T) {
	t.Run("reconnect on watch err", func(t *testing.T) {
		fx := newFixture(t)
		fx.testServer.watchErrOnce = true
		fx.run(t)
		defer fx.Finish()
		fx.testServer.waitStream(t)
		fx.testServer.releaseStream <- nil
	})
	t.Run("reconnect on start", func(t *testing.T) {
		fx := newFixture(t)
		fx.a.MustComponent(pool.CName).(*rpctest.TestPool).WithServer(nil)
		fx.run(t)
		defer fx.Finish()
		time.Sleep(time.Millisecond * 50)
		fx.a.MustComponent(pool.CName).(*rpctest.TestPool).WithServer(fx.drpcTS)
		fx.testServer.waitStream(t)
		fx.testServer.releaseStream <- nil
	})
}

func TestService_AddLog(t *testing.T) {
	fx := newFixture(t).run(t)
	defer fx.Finish()
	assert.NoError(t, fx.AddLog(ctx, &consensusproto.Log{}))
}

func TestService_AddRecord(t *testing.T) {
	fx := newFixture(t).run(t)
	defer fx.Finish()
	rec, err := fx.AddRecord(ctx, "1", &consensusproto.RawRecord{})
	require.NoError(t, err)
	assert.NotEmpty(t, rec)
}

var ctx = context.Background()

func newFixture(t *testing.T) *fixture {
	fx := &fixture{
		Service: New(),
		a:       &app.App{},
		ctrl:    gomock.NewController(t),
		testServer: &testServer{
			stream:        make(chan consensusproto.DRPCConsensus_LogWatchStream),
			releaseStream: make(chan error),
		},
	}
	fx.nodeconf = mock_nodeconf.NewMockService(fx.ctrl)
	fx.nodeconf.EXPECT().Name().Return(nodeconf.CName).AnyTimes()
	fx.nodeconf.EXPECT().Init(gomock.Any()).AnyTimes()
	fx.nodeconf.EXPECT().Run(gomock.Any()).AnyTimes()
	fx.nodeconf.EXPECT().Close(gomock.Any()).AnyTimes()
	fx.nodeconf.EXPECT().ConsensusPeers().Return([]string{"c1", "c2", "c3"}).AnyTimes()

	fx.drpcTS = rpctest.NewTestServer()
	require.NoError(t, consensusproto.DRPCRegisterConsensus(fx.drpcTS.Mux, fx.testServer))
	fx.a.Register(fx.Service).
		Register(&accounttest.AccountTestService{}).
		Register(fx.nodeconf).
		Register(rpctest.NewTestPool().WithServer(fx.drpcTS))

	return fx
}

type fixture struct {
	Service
	a          *app.App
	ctrl       *gomock.Controller
	testServer *testServer
	drpcTS     *rpctest.TestServer
	nodeconf   *mock_nodeconf.MockService
}

func (fx *fixture) run(t *testing.T) *fixture {
	require.NoError(t, fx.a.Start(ctx))
	return fx
}

func (fx *fixture) Finish() {
	assert.NoError(fx.ctrl.T, fx.a.Close(ctx))
	fx.ctrl.Finish()
}

type testServer struct {
	stream        chan consensusproto.DRPCConsensus_LogWatchStream
	addLog        func(ctx context.Context, req *consensusproto.LogAddRequest) error
	addRecord     func(ctx context.Context, req *consensusproto.RecordAddRequest) error
	releaseStream chan error
	watchErrOnce  bool
}

func (t *testServer) LogAdd(ctx context.Context, req *consensusproto.LogAddRequest) (*consensusproto.Ok, error) {
	if t.addLog != nil {
		if err := t.addLog(ctx, req); err != nil {
			return nil, err
		}
	}
	return &consensusproto.Ok{}, nil
}

func (t *testServer) RecordAdd(ctx context.Context, req *consensusproto.RecordAddRequest) (*consensusproto.RawRecordWithId, error) {
	if t.addRecord != nil {
		if err := t.addRecord(ctx, req); err != nil {
			return nil, err
		}
	}
	data, _ := req.Record.Marshal()
	id, _ := cidutil.NewCidFromBytes(data)
	return &consensusproto.RawRecordWithId{Id: id, Payload: data}, nil
}

func (t *testServer) LogWatch(stream consensusproto.DRPCConsensus_LogWatchStream) error {
	if t.watchErrOnce {
		t.watchErrOnce = false
		return fmt.Errorf("error")
	}
	t.stream <- stream
	return <-t.releaseStream
}

func (t *testServer) waitStream(test *testing.T) consensusproto.DRPCConsensus_LogWatchStream {
	select {
	case <-time.After(time.Second * 5):
		test.Fatalf("waiteStream timeout")
	case st := <-t.stream:
		return st
	}
	return nil
}

type testWatcher struct {
	recs  [][]*consensusproto.RawRecordWithId
	err   error
	ready chan struct{}
	once  sync.Once
}

func (t *testWatcher) AddConsensusRecords(recs []*consensusproto.RawRecordWithId) {
	t.recs = append(t.recs, recs)
	t.once.Do(func() {
		close(t.ready)
	})
}

func (t *testWatcher) AddConsensusError(err error) {
	t.err = err
	t.once.Do(func() {
		close(t.ready)
	})
}

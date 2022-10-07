package stream

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

var ctx = context.Background()

func TestService_NewStream(t *testing.T) {
	fx := newFixture(t)
	defer fx.Finish(t)

	var expLogId = []byte("logId")
	var preloadLogId = []byte("preloadId")

	fx.mockDB.fetchLog = func(ctx context.Context, logId []byte) (log consensus.Log, err error) {
		require.Equal(t, expLogId, logId)
		return consensus.Log{
			Id: logId,
			Records: []consensus.Record{
				{
					Id: []byte{'1'},
				},
			},
		}, nil
	}

	fx.mockDB.receiver(preloadLogId, []consensus.Record{
		{
			Id:     []byte{'2'},
			PrevId: []byte{'1'},
		},
		{
			Id: []byte{'1'},
		},
	})

	st1 := fx.NewStream()
	sr1 := readStream(st1)
	assert.Equal(t, uint64(1), sr1.id)
	st1.WatchIds(ctx, [][]byte{expLogId, preloadLogId})
	st1.UnwatchIds(ctx, [][]byte{preloadLogId})
	assert.Equal(t, [][]byte{expLogId}, st1.LogIds())

	st2 := fx.NewStream()
	sr2 := readStream(st2)
	assert.Equal(t, uint64(2), sr2.id)
	st2.WatchIds(ctx, [][]byte{expLogId, preloadLogId})

	fx.mockDB.receiver(expLogId, []consensus.Record{
		{
			Id: []byte{'1'},
		},
	})
	fx.mockDB.receiver(expLogId, []consensus.Record{
		{
			Id:     []byte{'2'},
			PrevId: []byte{'1'},
		},
		{
			Id: []byte{'1'},
		},
	})
	fx.mockDB.receiver(preloadLogId, []consensus.Record{
		{
			Id:     []byte{'3'},
			PrevId: []byte{'4'},
		},
		{
			Id:     []byte{'2'},
			PrevId: []byte{'1'},
		},
		{
			Id: []byte{'1'},
		},
	})
	st1.Close()
	st2.Close()

	for _, sr := range []*streamReader{sr1, sr2} {
		select {
		case <-time.After(time.Second / 3):
			require.False(t, true, "timeout")
		case <-sr.finished:
		}
	}

	require.Len(t, sr1.logs, 2)
	assert.Len(t, sr1.logs[string(expLogId)].Records, 2)
	assert.Equal(t, []byte{'2'}, sr1.logs[string(expLogId)].Records[0].Id)
	assert.Equal(t, []byte{'2'}, sr1.logs[string(preloadLogId)].Records[0].Id)

	require.Len(t, sr2.logs, 2)
	assert.Len(t, sr2.logs[string(expLogId)].Records, 2)
	assert.Equal(t, []byte{'2'}, sr2.logs[string(expLogId)].Records[0].Id)
	assert.Equal(t, []byte{'3'}, sr2.logs[string(preloadLogId)].Records[0].Id)
}

func newFixture(t *testing.T) *fixture {
	fx := &fixture{
		Service: New(),
		mockDB:  &mockDB{},
		a:       new(app.App),
	}

	fx.a.Register(fx.Service).Register(fx.mockDB)
	require.NoError(t, fx.a.Start(ctx))
	return fx
}

type fixture struct {
	Service
	mockDB *mockDB
	a      *app.App
}

func (fx *fixture) Finish(t *testing.T) {
	require.NoError(t, fx.a.Close(ctx))
}

func readStream(st *Stream) *streamReader {
	sr := &streamReader{
		id:       st.id,
		stream:   st,
		logs:     map[string]consensus.Log{},
		finished: make(chan struct{}),
	}
	go sr.read()
	return sr
}

type streamReader struct {
	id     uint64
	stream *Stream

	logs     map[string]consensus.Log
	finished chan struct{}
}

func (sr *streamReader) read() {
	defer close(sr.finished)
	for {
		logs := sr.stream.WaitLogs()
		if len(logs) == 0 {
			return
		}
		for _, l := range logs {
			if el, ok := sr.logs[string(l.Id)]; !ok {
				sr.logs[string(l.Id)] = l
			} else {
				el.Records = append(l.Records, el.Records...)
				sr.logs[string(l.Id)] = el
			}
		}
	}
}

type mockDB struct {
	receiver db.ChangeReceiver
	fetchLog func(ctx context.Context, logId []byte) (log consensus.Log, err error)
}

func (m *mockDB) AddLog(ctx context.Context, log consensus.Log) (err error) { return nil }
func (m *mockDB) AddRecord(ctx context.Context, logId []byte, record consensus.Record) (err error) {
	return nil
}

func (m *mockDB) FetchLog(ctx context.Context, logId []byte) (log consensus.Log, err error) {
	return m.fetchLog(ctx, logId)
}

func (m *mockDB) SetChangeReceiver(receiver db.ChangeReceiver) (err error) {
	m.receiver = receiver
	return nil
}

func (m *mockDB) Init(a *app.App) (err error) {
	return nil
}

func (m *mockDB) Name() (name string) {
	return db.CName
}

func (m *mockDB) Run(ctx context.Context) (err error) {
	return
}

func (m *mockDB) Close(ctx context.Context) (err error) {
	return
}

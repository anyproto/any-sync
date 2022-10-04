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

func TestService_Subscribe(t *testing.T) {
	fx := newFixture(t)
	defer fx.Finish(t)

	var expLogId = []byte("logId")

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

	st1, err := fx.Subscribe(ctx, expLogId)
	require.NoError(t, err)
	require.Equal(t, expLogId, st1.LogId())
	sr1 := readStream(st1)
	assert.Equal(t, uint32(1), sr1.id)

	st2, err := fx.Subscribe(ctx, expLogId)
	require.NoError(t, err)
	require.Equal(t, expLogId, st2.LogId())
	sr2 := readStream(st2)
	assert.Equal(t, uint32(2), sr2.id)

	fx.mockDB.receiver(expLogId, []consensus.Record{
		{
			Id: []byte{'1'},
		},
	})
	fx.mockDB.receiver([]byte("other id"), []consensus.Record{
		{
			Id:     []byte{'2'},
			PrevId: []byte{'1'},
		},
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
	st1.Close()
	st2.Close()

	for _, sr := range []*streamReader{sr1, sr2} {
		select {
		case <-time.After(time.Second / 3):
			require.False(t, true, "timeout")
		case <-sr.finished:
		}
	}

	require.Equal(t, sr1.records, sr2.records)
	require.Len(t, sr1.records, 1)
	assert.Equal(t, []byte{'2'}, sr1.records[0].Id)
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

func readStream(st Stream) *streamReader {
	sr := &streamReader{
		id:       st.(*stream).id,
		stream:   st,
		finished: make(chan struct{}),
	}
	go sr.read()
	return sr
}

type streamReader struct {
	id     uint32
	stream Stream

	records  []consensus.Record
	finished chan struct{}
}

func (sr *streamReader) read() {
	defer close(sr.finished)
	for {
		records := sr.stream.WaitRecords()
		if len(records) == 0 {
			return
		}
		sr.records = append(sr.records, records...)
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

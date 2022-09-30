package db

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto/consensuserrs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

var ctx = context.Background()

func TestService_AddLog(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fx := newFixture(t, nil)
		defer fx.Finish(t)
		log := consensus.Log{
			Id: []byte("logOne"),
			Records: []consensus.Record{
				{
					Id:      []byte("recordOne"),
					PrevId:  nil,
					Payload: []byte("payload"),
					Created: time.Now().Truncate(time.Second).UTC(),
				},
			},
		}
		require.NoError(t, fx.AddLog(ctx, log))
		fetched, err := fx.FetchLog(ctx, log.Id)
		require.NoError(t, err)
		assert.Equal(t, log, fetched)
	})
	t.Run("duplicate error", func(t *testing.T) {
		fx := newFixture(t, nil)
		defer fx.Finish(t)
		log := consensus.Log{
			Id: []byte("logOne"),
		}
		require.NoError(t, fx.AddLog(ctx, log))
		// TODO: check for specified error
		require.Error(t, fx.AddLog(ctx, log))
	})
}

func TestService_AddRecord(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fx := newFixture(t, nil)
		defer fx.Finish(t)
		var records = []consensus.Record{
			{
				Id:     []byte("2"),
				PrevId: []byte("1"),
			},
			{
				Id:     []byte("3"),
				PrevId: []byte("2"),
			},
			{
				Id:     []byte("4"),
				PrevId: []byte("3"),
			},
		}
		l := consensus.Log{
			Id: []byte("logTestRecords"),
			Records: []consensus.Record{
				{
					Id: []byte("1"),
				},
			},
		}
		require.NoError(t, fx.AddLog(ctx, l))
		for _, rec := range records {
			require.NoError(t, fx.AddRecord(ctx, l.Id, rec))
		}
		fx.assertLogValid(t, l.Id, 4)
	})
	t.Run("conflict", func(t *testing.T) {
		fx := newFixture(t, nil)
		defer fx.Finish(t)
		log := consensus.Log{
			Id: []byte("logTestRecords"),
			Records: []consensus.Record{
				{
					Id: []byte("1"),
				},
			},
		}
		require.NoError(t, fx.AddLog(ctx, log))
		assert.Error(t, fx.AddRecord(ctx, log.Id, consensus.Record{Id: []byte("2"), PrevId: []byte("3")}))
	})
}

func TestService_FetchLog(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		fx := newFixture(t, nil)
		defer fx.Finish(t)
		l, err := fx.FetchLog(ctx, []byte("not exists"))
		assert.Empty(t, l)
		assert.ErrorIs(t, err, consensuserrs.ErrLogNotFound)
	})
}

func TestService_ChangeReceive(t *testing.T) {
	t.Run("set after run", func(t *testing.T) {
		fx := newFixture(t, nil)
		defer fx.Finish(t)
		assert.Error(t, fx.SetChangeReceiver(func(logId []byte, records []consensus.Record) {}))
	})
	t.Run("receive changes", func(t *testing.T) {
		var logs = make(chan consensus.Log, 10)
		var count int
		fx := newFixture(t, func(logId []byte, records []consensus.Record) {
			logs <- consensus.Log{Id: logId, Records: records}
			count++
		})
		defer fx.Finish(t)
		var l = consensus.Log{
			Id: []byte("logTestStream"),
			Records: []consensus.Record{
				{
					Id: []byte("1"),
				},
			},
		}
		var records = []consensus.Record{
			{
				Id:     []byte("2"),
				PrevId: []byte("1"),
			},
			{
				Id:     []byte("3"),
				PrevId: []byte("2"),
			},
			{
				Id:     []byte("4"),
				PrevId: []byte("3"),
			},
		}
		require.NoError(t, fx.AddLog(ctx, l))
		assert.Empty(t, count)

		for _, rec := range records {
			require.NoError(t, fx.AddRecord(ctx, l.Id, rec))
		}

		timeout := time.After(time.Second)
		for i := 0; i < len(records); i++ {
			select {
			case resLog := <-logs:
				assertLogValid(t, resLog, i+2)
			case <-timeout:
				require.False(t, true)
			}
		}
	})
}

func newFixture(t *testing.T, cr ChangeReceiver) *fixture {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	fx := &fixture{
		Service: New(),
		cancel:  cancel,
		a:       new(app.App),
	}
	fx.a.Register(&config.Config{
		Mongo: config.Mongo{
			Connect:       "mongodb://localhost:27017/?w=majority",
			Database:      "consensus_test",
			LogCollection: "log",
		},
	})
	fx.a.Register(fx.Service)
	require.NoError(t, fx.Service.SetChangeReceiver(cr))
	err := fx.a.Start(ctx)
	if err != nil {
		fx.cancel()
	}
	require.NoError(t, err)
	return fx
}

type fixture struct {
	Service
	a      *app.App
	cancel context.CancelFunc
}

func (fx *fixture) Finish(t *testing.T) {
	if fx.cancel != nil {
		fx.cancel()
	}
	coll := fx.Service.(*service).logColl
	t.Log(coll.Drop(ctx))
	assert.NoError(t, fx.a.Close(ctx))
}

func (fx *fixture) assertLogValid(t *testing.T, logId []byte, count int) {
	log, err := fx.FetchLog(ctx, logId)
	require.NoError(t, err)
	assertLogValid(t, log, count)
}

func assertLogValid(t *testing.T, log consensus.Log, count int) {
	if count >= 0 {
		assert.Len(t, log.Records, count)
	}
	var prevId []byte
	for _, rec := range log.Records {
		if len(prevId) != 0 {
			assert.Equal(t, string(prevId), string(rec.Id))
		}
		prevId = rec.PrevId
	}
}

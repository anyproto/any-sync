package acl

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/testutil/testaccount"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusclient"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusclient/mock_consensusclient"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

var ctx = context.Background()

func TestService_CreateLog(t *testing.T) {
	fx := newFixture(t)
	defer fx.Finish(t)
	var clog *consensusproto.Log
	fx.mockClient.EXPECT().AddLog(ctx, gomock.Any()).Do(func(ctx context.Context, l *consensusproto.Log) {
		clog = l
	})

	aclId, _ := cid.NewCIDFromBytes([]byte("aclId"))

	rec := &aclrecordproto.ACLRecord{
		PrevId:    "",
		Identity:  fx.account.Account().Identity,
		Data:      []byte{'1', '2', '3'},
		Timestamp: time.Now().Unix(),
	}
	pl, _ := rec.Marshal()

	firstRecId, err := fx.CreateLog(ctx, aclId, &aclrecordproto.RawACLRecord{
		Payload: pl,
	})
	require.NoError(t, err)
	aclIdBytes, _ := cidToByte(aclId)
	firstRecIdBytes, _ := cidToByte(firstRecId)
	assert.Equal(t, aclIdBytes, clog.Id)
	assert.NotEmpty(t, firstRecIdBytes)
	require.Len(t, clog.Records, 1)

	var resultRawAcl = &aclrecordproto.RawACLRecord{}
	require.NoError(t, resultRawAcl.Unmarshal(clog.Records[0].Payload))
	valid, err := fx.account.Account().SignKey.GetPublic().Verify(resultRawAcl.Payload, resultRawAcl.AcceptorSignature)
	require.NoError(t, err)
	require.True(t, valid)
}

func TestService_AddRecord(t *testing.T) {
	fx := newFixture(t)
	defer fx.Finish(t)
	var clog *consensusproto.Log
	fx.mockClient.EXPECT().AddLog(ctx, gomock.Any()).Do(func(ctx context.Context, l *consensusproto.Log) {
		clog = l
	})

	aclId, _ := cid.NewCIDFromBytes([]byte("aclId"))

	rec := &aclrecordproto.ACLRecord{
		PrevId:    "",
		Identity:  fx.account.Account().Identity,
		Data:      []byte{'1', '2', '3'},
		Timestamp: time.Now().Unix(),
	}
	pl, _ := rec.Marshal()

	firstRecId, err := fx.CreateLog(ctx, aclId, &aclrecordproto.RawACLRecord{
		Payload: pl,
	})
	require.NoError(t, err)
	aclIdBytes, _ := cidToByte(aclId)
	firstRecIdBytes, _ := cidToByte(firstRecId)
	assert.Equal(t, aclIdBytes, clog.Id)
	assert.NotEmpty(t, firstRecIdBytes)
	var addRec *consensusproto.Record
	fx.mockClient.EXPECT().AddRecord(ctx, aclIdBytes, gomock.Any()).Do(func(ctx context.Context, logId []byte, rec *consensusproto.Record) {
		addRec = rec
	})
	rec = &aclrecordproto.ACLRecord{
		PrevId:    firstRecId,
		Identity:  fx.account.Account().Identity,
		Data:      []byte{'1', '2', '3', '4'},
		Timestamp: time.Now().Unix(),
	}
	pl, _ = rec.Marshal()

	newRecId, err := fx.AddRecord(ctx, aclId, &aclrecordproto.RawACLRecord{
		Payload: pl,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, newRecId)

	assert.Equal(t, firstRecIdBytes, addRec.PrevId)

}

func TestService_Watch(t *testing.T) {
	t.Run("remote error", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish(t)
		var expErr = fmt.Errorf("error")
		aclId, _ := cid.NewCIDFromBytes([]byte("aclId"))
		aclIdBytes, _ := cidToByte(aclId)
		fx.mockClient.EXPECT().Watch(aclIdBytes, gomock.Any()).Do(func(aid []byte, w consensusclient.Watcher) {
			assert.Equal(t, aclIdBytes, aid)
			go func() {
				time.Sleep(time.Millisecond * 10)
				w.AddConsensusError(expErr)
			}()
		})

		th := &testHandler{}
		err := fx.Watch(ctx, "123", aclId, th)
		assert.Equal(t, expErr, err)
	})
	t.Run("success", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish(t)
		aclId, _ := cid.NewCIDFromBytes([]byte("aclId"))
		aclIdBytes, _ := cidToByte(aclId)
		fx.mockClient.EXPECT().Watch(aclIdBytes, gomock.Any()).Do(func(aid []byte, w consensusclient.Watcher) {
			assert.Equal(t, aclIdBytes, aid)
			go func() {
				time.Sleep(time.Millisecond * 10)
				r1cid, _ := cid.NewCIDFromBytes([]byte("r1"))
				r2cid, _ := cid.NewCIDFromBytes([]byte("r2"))
				r1cidB, _ := cidToByte(r1cid)
				r2cidB, _ := cidToByte(r2cid)
				w.AddConsensusRecords([]*consensusproto.Record{
					{
						Id:      r2cidB,
						PrevId:  r1cidB,
						Payload: []byte("p1"),
					},
					{
						Id:      r1cidB,
						Payload: []byte("p1"),
					},
				})
			}()
		})

		th := &testHandler{}
		err := fx.Watch(ctx, "123", aclId, th)
		require.NoError(t, err)
	})
}

func newFixture(t *testing.T) *fixture {
	fx := &fixture{
		a:       new(app.App),
		ctrl:    gomock.NewController(t),
		account: &testaccount.AccountTestService{},
	}
	fx.mockClient = mock_consensusclient.NewMockService(fx.ctrl)
	fx.mockClient.EXPECT().Name().Return(consensusclient.CName).AnyTimes()
	fx.mockClient.EXPECT().Init(gomock.Any()).AnyTimes()
	fx.mockClient.EXPECT().Run(gomock.Any()).AnyTimes()
	fx.mockClient.EXPECT().Close(gomock.Any()).AnyTimes()
	fx.Service = New()
	fx.a.Register(fx.account).Register(fx.mockClient).Register(fx.Service)
	require.NoError(t, fx.a.Start(ctx))
	return fx
}

type fixture struct {
	Service
	mockClient *mock_consensusclient.MockService
	ctrl       *gomock.Controller
	a          *app.App
	account    *testaccount.AccountTestService
}

func (fx *fixture) Finish(t *testing.T) {
	require.NoError(t, fx.a.Close(ctx))
	fx.ctrl.Finish()
}

type testHandler struct {
	req *spacesyncproto.ObjectSyncMessage
}

func (t *testHandler) HandleMessage(ctx context.Context, senderId string, request *spacesyncproto.ObjectSyncMessage) (err error) {
	t.req = request
	return
}

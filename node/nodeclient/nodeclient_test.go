package nodeclient

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto/mock_spacesyncproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

var ctx = context.Background()

type fixture struct {
	*nodeClient
	ctrl *gomock.Controller
}

func (f *fixture) finish() {
	f.ctrl.Finish()
}

func newFixture(t *testing.T) *fixture {
	ctrl := gomock.NewController(t)
	return &fixture{
		nodeClient: New().(*nodeClient),
		ctrl:       ctrl,
	}
}

func TestNodeClient_AclGetRecords(t *testing.T) {
	f := newFixture(t)
	defer f.finish()

	spaceId := "spaceId"
	aclHead := "aclHead"
	cl := mock_spacesyncproto.NewMockDRPCSpaceSyncClient(f.ctrl)
	clientDo = func(client *nodeClient, ctx context.Context, s string, f func(cl spacesyncproto.DRPCSpaceSyncClient) error) error {
		return f(cl)
	}
	var (
		expectedRecs     []*consensusproto.RawRecordWithId
		expectedByteRecs [][]byte
		total            = 5
	)
	for i := 0; i < total; i++ {
		expectedRecs = append(expectedRecs, &consensusproto.RawRecordWithId{
			Id: fmt.Sprint(i),
		})
		marshalled, err := expectedRecs[i].MarshalVT()
		require.NoError(t, err)
		expectedByteRecs = append(expectedByteRecs, marshalled)
	}
	cl.EXPECT().AclGetRecords(ctx, &spacesyncproto.AclGetRecordsRequest{
		SpaceId: spaceId,
		AclHead: aclHead,
	}).Return(&spacesyncproto.AclGetRecordsResponse{Records: expectedByteRecs}, nil)
	recs, err := f.AclGetRecords(ctx, spaceId, aclHead)
	require.NoError(t, err)
	require.Equal(t, expectedRecs, recs)
}

func TestNodeClient_AclAddRecords(t *testing.T) {
	f := newFixture(t)
	defer f.finish()

	spaceId := "spaceId"
	cl := mock_spacesyncproto.NewMockDRPCSpaceSyncClient(f.ctrl)
	clientDo = func(client *nodeClient, ctx context.Context, s string, f func(cl spacesyncproto.DRPCSpaceSyncClient) error) error {
		return f(cl)
	}
	sendRec := &consensusproto.RawRecord{
		AcceptorTimestamp: 10,
	}
	data, err := sendRec.MarshalVT()
	require.NoError(t, err)
	expectedRec := &consensusproto.RawRecordWithId{
		Id:      "recId",
		Payload: data,
	}
	cl.EXPECT().AclAddRecord(ctx, &spacesyncproto.AclAddRecordRequest{
		SpaceId: spaceId,
		Payload: data,
	}).Return(&spacesyncproto.AclAddRecordResponse{RecordId: expectedRec.Id, Payload: expectedRec.Payload}, nil)
	rec, err := f.AclAddRecord(ctx, spaceId, sendRec)
	require.NoError(t, err)
	require.Equal(t, expectedRec, rec)
}

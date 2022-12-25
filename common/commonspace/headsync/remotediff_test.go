package headsync

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/ldiff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRemote(t *testing.T) {
	ldLocal := ldiff.New(8, 8)
	ldRemote := ldiff.New(8, 8)
	for i := 0; i < 100; i++ {
		el := ldiff.Element{
			Id:   fmt.Sprint(i),
			Head: fmt.Sprint(i),
		}
		ldRemote.Set(el)
		if i%10 != 0 {
			ldLocal.Set(el)
		}
	}

	rd := NewRemoteDiff("1", &mockClient{l: ldRemote})
	newIds, changedIds, removedIds, err := ldLocal.Diff(context.Background(), rd)
	require.NoError(t, err)
	assert.Len(t, newIds, 10)
	assert.Len(t, changedIds, 0)
	assert.Len(t, removedIds, 0)
}

type mockClient struct {
	l ldiff.Diff
}

func (m *mockClient) HeadSync(ctx context.Context, in *spacesyncproto.HeadSyncRequest) (*spacesyncproto.HeadSyncResponse, error) {
	return HandleRangeRequest(ctx, m.l, in)
}

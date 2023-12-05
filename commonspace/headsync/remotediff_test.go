package headsync

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/app/ldiff"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

func TestRemote(t *testing.T) {
	ldLocal := ldiff.New(32, 256)
	ldRemote := ldiff.New(32, 256)
	var (
		localEls  []ldiff.Element
		remoteEls []ldiff.Element
	)

	for i := 0; i < 100000; i++ {
		el := ldiff.Element{
			Id:   fmt.Sprint(i),
			Head: fmt.Sprint(i),
		}
		remoteEls = append(remoteEls, el)
		if i%100 == 0 {
			localEls = append(localEls, el)
		}
	}
	ldLocal.Set(localEls...)
	ldRemote.Set(remoteEls...)

	rd := NewRemoteDiff("1", &mockClient{l: ldRemote})
	newIds, changedIds, removedIds, err := ldLocal.Diff(context.Background(), rd)
	require.NoError(t, err)
	assert.Len(t, newIds, 99000)
	assert.Len(t, changedIds, 0)
	assert.Len(t, removedIds, 0)
}

type mockClient struct {
	l ldiff.Diff
}

func (m *mockClient) HeadSync(ctx context.Context, in *spacesyncproto.HeadSyncRequest) (*spacesyncproto.HeadSyncResponse, error) {
	return HandleRangeRequest(ctx, m.l, in)
}

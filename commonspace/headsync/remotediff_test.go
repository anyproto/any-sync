package headsync

import (
	"context"
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/app/ldiff"
	"github.com/anyproto/any-sync/app/olddiff"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

func benchmarkDifferentDiffs(t *testing.T, diffFactory func() ldiff.Diff, headLength int) {
	moduloValues := []int{1, 10, 100, 1000, 10000, 100000}
	totalElements := 100000

	for _, modVal := range moduloValues {
		t.Run(fmt.Sprintf("New_%d", totalElements/modVal), func(t *testing.T) {
			// Create a new diff instance for each test using the factory
			contLocal := diffFactory()
			contRemote := diffFactory()
			remClient := &mockClient{t: t, l: contRemote}

			var (
				localEls  []ldiff.Element
				remoteEls []ldiff.Element
			)

			buf := make([]byte, headLength)
			_, _ = rand.Read(buf)

			for i := 0; i < totalElements; i++ {
				el := ldiff.Element{
					Id:   fmt.Sprint(i),
					Head: string(buf),
				}
				remoteEls = append(remoteEls, el)
				if i%modVal != 0 {
					localEls = append(localEls, el)
				}
			}

			contLocal.Set(localEls...)
			remClient.l.Set(remoteEls...)

			rd := NewRemoteDiff("1", remClient)
			newIds, changedIds, removedIds, err := contLocal.Diff(context.Background(), rd)
			require.NoError(t, err)

			expectedNewCount := totalElements / modVal
			assert.Len(t, newIds, expectedNewCount)
			assert.Len(t, changedIds, 0)
			assert.Len(t, removedIds, 0)

			fmt.Printf("New count %d: total bytes sent: %d, %d\n", expectedNewCount, remClient.totalInSent, remClient.totalOutSent)
		})
	}
}

func TestBenchRemoteWithDifferentCounts(t *testing.T) {
	t.Run("StandardLdiff", func(t *testing.T) {
		benchmarkDifferentDiffs(t, func() ldiff.Diff {
			return ldiff.New(32, 256)
		}, 32)
	})
	//old has higher head lengths because of hashes
	t.Run("OldLdiff", func(t *testing.T) {
		benchmarkDifferentDiffs(t, func() ldiff.Diff {
			return olddiff.New(32, 256)
		}, 100)
	})
}

type mockClient struct {
	l            ldiff.Diff
	totalInSent  int
	totalOutSent int
	t            *testing.T
}

func (m *mockClient) HeadSync(ctx context.Context, in *spacesyncproto.HeadSyncRequest) (*spacesyncproto.HeadSyncResponse, error) {
	res, err := in.MarshalVT()
	require.NoError(m.t, err)
	m.totalInSent += len(res)
	resp, err := HandleRangeRequest(ctx, m.l, in)
	if err != nil {
		return nil, err
	}
	marsh, err := resp.MarshalVT()
	require.NoError(m.t, err)
	m.totalOutSent += len(marsh)
	return resp, nil
}

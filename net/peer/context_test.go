package peer

import (
	"context"
	"time"

	"testing"

	"github.com/stretchr/testify/require"
)

func TestCtxProtoVersion(t *testing.T) {
	ctx := CtxWithProtoVersion(ctx, 1)
	ver, err := CtxProtoVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, uint32(1), ver)
}

func TestCtxTTL(t *testing.T) {
	assert.Zero(t, CtxTTL(context.Background()))
	assert.Equal(t, time.Minute, CtxTTL(CtxWithTTL(context.Background(), time.Minute)))
}

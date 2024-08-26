package peer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCtxProtoVersion(t *testing.T) {
	ctx := CtxWithProtoVersion(ctx, 1)
	ver, err := CtxProtoVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, uint32(1), ver)
}

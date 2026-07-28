package streampool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/app/debugstat"
)

// app.Start closes components it never ran: a failing Init makes closeServices
// call Close on every component registered before it.
func TestStreamPool_CloseWithoutRun(t *testing.T) {
	s := New().(*streamPool)
	s.statService = debugstat.NewNoOp()
	require.NotPanics(t, func() {
		require.NoError(t, s.Close(context.Background()))
	})
}

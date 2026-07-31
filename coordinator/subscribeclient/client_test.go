package subscribeclient

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
)

// app.Start closes components it never ran: a failing Init makes closeServices
// call Close on every component registered before it. s.close is only closed by
// streamWatcher, which Run starts.
func TestSubscribeClient_CloseWithoutRun(t *testing.T) {
	s := &subscribeClient{}
	s.ctx, s.ctxCancel = context.WithCancel(context.Background())
	s.close = make(chan struct{})
	s.callbacks = make(map[coordinatorproto.NotifyEventType]EventCallback)

	closed := make(chan struct{})
	go func() {
		require.NoError(t, s.Close(context.Background()))
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		require.Fail(t, "Close deadlocked when Run was never called")
	}
}

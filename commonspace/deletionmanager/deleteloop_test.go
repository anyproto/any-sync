package deletionmanager

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDeleteLoop_CloseCancelsDeleteCtx(t *testing.T) {
	var (
		started = make(chan struct{})
		done    = make(chan struct{})
	)
	dl := newDeleteLoop(func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		close(done)
	})
	dl.Run()
	<-started
	dl.Close(context.Background())
	select {
	case <-done:
	default:
		require.Fail(t, "deleteFunc was not cancelled")
	}
}

func TestDeleteLoop_CloseBoundedWhenDeleteFuncIgnoresCtx(t *testing.T) {
	var (
		started = make(chan struct{})
		release = make(chan struct{})
	)
	dl := newDeleteLoop(func(ctx context.Context) {
		close(started)
		<-release
	})
	dl.closeTimeout = 10 * time.Millisecond
	dl.Run()
	<-started
	closed := make(chan struct{})
	go func() {
		dl.Close(context.Background())
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		require.Fail(t, "Close blocked on a deleteFunc that ignores ctx")
	}
	close(release)
}

func TestDeleteLoop_CloseHonoursCtx(t *testing.T) {
	var (
		started = make(chan struct{})
		release = make(chan struct{})
	)
	dl := newDeleteLoop(func(ctx context.Context) {
		close(started)
		<-release
	})
	dl.Run()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	closed := make(chan struct{})
	go func() {
		dl.Close(ctx)
		close(closed)
	}()
	cancel()
	select {
	case <-closed:
	case <-time.After(time.Second):
		require.Fail(t, "Close ignored the cancelled ctx")
	}
	close(release)
}

// app.Start closes components it never ran: loopDone is only closed by the loop.
func TestDeleteLoop_CloseWithoutRun(t *testing.T) {
	dl := newDeleteLoop(func(ctx context.Context) {})
	closed := make(chan struct{})
	go func() {
		dl.Close(context.Background())
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		require.Fail(t, "Close waited for a loop that was never started")
	}
}

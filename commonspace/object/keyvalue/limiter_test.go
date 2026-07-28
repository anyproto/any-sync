package keyvalue

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConcurrentLimiter_CloseBoundedWhenActionIgnoresCtx(t *testing.T) {
	var (
		started = make(chan struct{})
		release = make(chan struct{})
	)
	cl := newConcurrentLimiter()
	cl.closeTimeout = 10 * time.Millisecond
	require.True(t, cl.ScheduleRequest(context.Background(), "peer", func() {
		close(started)
		<-release
	}))
	<-started
	closed := make(chan struct{})
	go func() {
		cl.Close(context.Background())
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		require.Fail(t, "Close blocked on a request that ignores ctx")
	}
	close(release)
}

func TestConcurrentLimiter_CloseHonoursCtx(t *testing.T) {
	var (
		started = make(chan struct{})
		release = make(chan struct{})
	)
	cl := newConcurrentLimiter()
	require.True(t, cl.ScheduleRequest(context.Background(), "peer", func() {
		close(started)
		<-release
	}))
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	closed := make(chan struct{})
	go func() {
		cl.Close(ctx)
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

// A timed-out Close leaves a waiter parked on the WaitGroup: a later Add would
// panic with "WaitGroup is reused before previous Wait has returned".
func TestConcurrentLimiter_NoScheduleAfterClose(t *testing.T) {
	for i := 0; i < 500; i++ {
		cl := newConcurrentLimiter()
		cl.closeTimeout = time.Microsecond
		release := make(chan struct{})
		require.True(t, cl.ScheduleRequest(context.Background(), "peer", func() {
			<-release
		}))
		cl.Close(context.Background())

		var start sync.WaitGroup
		start.Add(2)
		go func() {
			start.Done()
			start.Wait()
			close(release)
		}()
		go func() {
			start.Done()
			start.Wait()
			require.False(t, cl.ScheduleRequest(context.Background(), "peer2", func() {}))
		}()
	}
}

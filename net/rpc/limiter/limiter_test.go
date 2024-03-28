package limiter

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/net/peer"
)

var ctx = context.Background()

type mockHandler struct {
	calls atomic.Int32
}

type mockStream struct {
	ctx context.Context
}

func (m mockStream) Context() context.Context {
	return m.ctx
}

func (m mockStream) MsgSend(msg drpc.Message, enc drpc.Encoding) error {
	return nil
}

func (m mockStream) MsgRecv(msg drpc.Message, enc drpc.Encoding) error {
	return nil
}

func (m mockStream) CloseSend() error {
	return nil
}

func (m mockStream) Close() error {
	return nil
}

func (m *mockHandler) HandleRPC(stream drpc.Stream, rpc string) (err error) {
	m.calls.Add(1)
	return nil
}

func TestLimiter_Synchronous(t *testing.T) {
	lim := New().(*limiter)
	handler := &mockHandler{}
	lim.cfg = Config{
		DefaultTokens: Tokens{
			TokensPerSecond: 100,
			MaxTokens:       100,
		},
		ResponseTokens: map[string]Tokens{
			"rpc": {
				TokensPerSecond: 10,
				MaxTokens:       1,
			},
		},
	}
	lim.peerCheckInterval = 10 * time.Millisecond
	wrapped := lim.WrapDRPCHandler(handler)
	// rpc call allows only one token max, so it should let only first call
	// for second one we should wait 100 ms
	firstStream := mockStream{ctx: peer.CtxWithPeerId(ctx, "peer1")}
	// check that we are using specific timeout
	err := wrapped.HandleRPC(firstStream, "rpc")
	require.NoError(t, err)
	err = wrapped.HandleRPC(firstStream, "rpc")
	require.Equal(t, ErrLimitExceeded, err)
	// second stream should not affect the first one
	secondStream := mockStream{ctx: peer.CtxWithPeerId(ctx, "peer2")}
	err = wrapped.HandleRPC(secondStream, "rpc")
	require.NoError(t, err)
	// after 100 ms new token has been generated
	time.Sleep(100 * time.Millisecond)
	err = wrapped.HandleRPC(firstStream, "rpc")
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	// checking that peer loop cleaned the map
	err = lim.peerLoop(ctx)
	require.NoError(t, err)
	// now we should be able to call again, because we cleared the map
	err = wrapped.HandleRPC(firstStream, "rpc")
	require.NoError(t, err)
	// but limit of 1 sec is not enough to clean the map
	time.Sleep(1 * time.Millisecond)
	err = wrapped.HandleRPC(firstStream, "rpc")
	require.Equal(t, ErrLimitExceeded, err)
}

func TestLimiter_Concurrent_NoBursts(t *testing.T) {
	lim := New().(*limiter)
	handler := &mockHandler{}
	var (
		targetRps = 10
		// peerRps should be greater than targetRps
		peerRps  = 100
		reqDelay = time.Duration(1000/peerRps) * time.Millisecond
	)
	lim.cfg = Config{
		DefaultTokens: Tokens{
			TokensPerSecond: targetRps,
			MaxTokens:       1,
		},
	}
	wrapped := lim.WrapDRPCHandler(handler)
	firstStream := mockStream{ctx: peer.CtxWithPeerId(ctx, "peer1")}
	secondStream := mockStream{ctx: peer.CtxWithPeerId(ctx, "peer2")}
	waitFirst := make(chan struct{})
	waitSecond := make(chan struct{})
	go func() {
		for i := 0; i < peerRps; i++ {
			time.Sleep(reqDelay)
			_ = wrapped.HandleRPC(firstStream, "rpc")
		}
		close(waitFirst)
	}()
	go func() {
		for i := 0; i < peerRps; i++ {
			time.Sleep(reqDelay)
			_ = wrapped.HandleRPC(secondStream, "rpc")
		}
		close(waitSecond)
	}()
	<-waitFirst
	<-waitSecond
	// 2 for number of peers and 2 for error margin (delays etc)
	maxCalls := 2 * 2 * targetRps
	require.Greater(t, maxCalls, int(handler.calls.Load()))
}

func TestLimiter_Concurrent_Bursts(t *testing.T) {
	lim := New().(*limiter)
	handler := &mockHandler{}
	var (
		targetRps = 10
		// bursts are not affected by rps limit
		burst = 10
		// peerRps should be greater than targetRps + burst
		peerRps  = 100
		reqDelay = time.Duration(1000/peerRps) * time.Millisecond
	)
	lim.cfg = Config{
		DefaultTokens: Tokens{
			TokensPerSecond: targetRps,
			MaxTokens:       burst,
		},
	}
	wrapped := lim.WrapDRPCHandler(handler)
	firstStream := mockStream{ctx: peer.CtxWithPeerId(ctx, "peer1")}
	secondStream := mockStream{ctx: peer.CtxWithPeerId(ctx, "peer2")}
	waitFirst := make(chan struct{})
	waitSecond := make(chan struct{})
	go func() {
		for i := 0; i < peerRps; i++ {
			time.Sleep(reqDelay)
			_ = wrapped.HandleRPC(firstStream, "rpc")
		}
		close(waitFirst)
	}()
	go func() {
		for i := 0; i < peerRps; i++ {
			time.Sleep(reqDelay)
			_ = wrapped.HandleRPC(secondStream, "rpc")
		}
		close(waitSecond)
	}()
	<-waitFirst
	<-waitSecond
	// 2 for number of peers and 2 for error margin (delays etc)
	maxCalls := 2 * 2 * (targetRps + burst)
	minCalls := 2 * (targetRps + burst)
	require.Greater(t, maxCalls, int(handler.calls.Load()))
	require.LessOrEqual(t, minCalls, int(handler.calls.Load()))
}

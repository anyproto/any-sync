//go:generate mockgen -destination mock_limiter/mock_limiter.go github.com/anyproto/any-sync/net/rpc/limiter RpcLimiter
package limiter

import (
	"context"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/limiter/limiterproto"
	"github.com/anyproto/any-sync/util/periodicsync"
)

const (
	peerCheckInterval = 10 * time.Second
	checkTimeout      = 2 * time.Second
)

var log = logger.NewNamed(CName)

const CName = "common.rpc.limiter"

type RpcLimiter interface {
	app.ComponentRunnable
	// WrapDRPCHandler wraps the given drpc.Handler with additional functionality
	WrapDRPCHandler(handler drpc.Handler) drpc.Handler
}

func New() RpcLimiter {
	return &limiter{
		limiters:          make(map[string]*peerLimiter),
		peerCheckInterval: peerCheckInterval,
		checkTimeout:      checkTimeout,
	}
}

type peerLimiter struct {
	*rate.Limiter
	lastUsage time.Time
}

type limiter struct {
	drpc.Handler
	limiters          map[string]*peerLimiter
	periodicLoop      periodicsync.PeriodicSync
	peerCheckInterval time.Duration
	checkTimeout      time.Duration
	cfg               Config
	mx                sync.Mutex
}

func (h *limiter) Run(ctx context.Context) (err error) {
	h.periodicLoop.Run()
	return nil
}

func (h *limiter) Close(ctx context.Context) (err error) {
	h.periodicLoop.Close()
	return
}

func (h *limiter) Init(a *app.App) (err error) {
	h.periodicLoop = periodicsync.NewPeriodicSyncDuration(h.peerCheckInterval, h.checkTimeout, h.peerLoop, log)
	h.cfg = a.MustComponent("config").(ConfigGetter).GetLimiterConf()
	return nil
}

func (h *limiter) Name() (name string) {
	return CName
}

func (h *limiter) peerLoop(ctx context.Context) error {
	h.mx.Lock()
	defer h.mx.Unlock()
	for rpcPeer, lim := range h.limiters {
		if time.Since(lim.lastUsage) > h.peerCheckInterval {
			delete(h.limiters, rpcPeer)
		}
	}
	return nil
}

func (h *limiter) WrapDRPCHandler(handler drpc.Handler) drpc.Handler {
	h.mx.Lock()
	defer h.mx.Unlock()
	h.Handler = handler
	return h
}

func (h *limiter) HandleRPC(stream drpc.Stream, rpc string) (err error) {
	peerId, err := peer.CtxPeerId(stream.Context())
	if err != nil {
		return err
	}
	lim := h.getPeerLimiter(peerId, rpc)
	if !lim.Allow() {
		return limiterproto.ErrLimitExceeded
	}
	return h.Handler.HandleRPC(stream, rpc)
}

func (h *limiter) getLimits(rpc string) Tokens {
	if tokens, exists := h.cfg.ResponseTokens[rpc]; exists {
		return tokens
	}
	return h.cfg.DefaultTokens
}

func (h *limiter) getPeerLimiter(peerId string, rpc string) *peerLimiter {
	// rpc looks like this /anyNodeSync.NodeSync/PartitionSync
	rpcPeer := strings.Join([]string{peerId, rpc}, "-")
	h.mx.Lock()
	defer h.mx.Unlock()
	lim, ok := h.limiters[rpcPeer]
	if !ok {
		limits := h.getLimits(rpc)
		lim = &peerLimiter{
			Limiter: rate.NewLimiter(rate.Limit(limits.TokensPerSecond), limits.MaxTokens),
		}
		h.limiters[rpcPeer] = lim
	}
	lim.lastUsage = time.Now()
	return lim
}

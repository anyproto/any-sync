//go:generate mockgen -destination mock_periodicsync/mock_periodicsync.go github.com/anyproto/any-sync/util/periodicsync PeriodicSync
package periodicsync

import (
	"context"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/logger"
)

type PeriodicSync interface {
	Kick(ctx context.Context) error
	Reset(ctx context.Context) error
	ResetTimer()
	Run()
	Close()
}

type SyncerFunc func(ctx context.Context) error

func NewPeriodicSync(periodSeconds int, timeout time.Duration, caller SyncerFunc, l logger.CtxLogger) PeriodicSync {
	return NewPeriodicSyncDuration(time.Duration(periodSeconds)*time.Second, timeout, caller, l)
}

func NewPeriodicSyncDuration(periodicLoopInterval, timeout time.Duration, caller SyncerFunc, l logger.CtxLogger) PeriodicSync {
	ctx, cancel := context.WithCancel(context.Background())
	ctx = logger.CtxWithFields(ctx, zap.String("rootOp", "periodicCall"))
	return &periodicCall{
		caller:     caller,
		log:        l,
		loopCtx:    ctx,
		loopCancel: cancel,
		loopDone:   make(chan struct{}),
		loopKick:   make(chan bool),
		loopReset:  make(chan struct{}),
		period:     periodicLoopInterval,
		timeout:    timeout,
	}
}

type periodicCall struct {
	log        logger.CtxLogger
	caller     SyncerFunc
	loopCtx    context.Context
	loopCancel context.CancelFunc
	loopDone   chan struct{}
	loopKick   chan bool
	loopReset  chan struct{}
	period     time.Duration
	timeout    time.Duration
	isRunning  atomic.Bool
}

func (p *periodicCall) Run() {
	p.isRunning.Store(true)
	go p.loop(p.period)
}

func (p *periodicCall) loop(period time.Duration) {
	defer close(p.loopDone)
	doCall := func() {
		ctx := p.loopCtx
		if p.timeout != 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(p.loopCtx, p.timeout)
			defer cancel()
		}
		if err := p.caller(ctx); err != nil {
			p.log.Warn("periodic call error", zap.Error(err))
		}
	}
	doCall()
	if period > 0 {
		ticker := time.NewTicker(period)
		defer ticker.Stop()
		for {
			select {
			case <-p.loopCtx.Done():
				return
			case kickReset := <-p.loopKick:
				if kickReset {
					ticker.Reset(period)
				}
				doCall()
			case <-p.loopReset:
				ticker.Reset(period)
			case <-ticker.C:
				doCall()
			}
		}
	}
}

// Kick runs the scheduled function once, without
// interrupting the schedule.
func (p *periodicCall) Kick(ctx context.Context) error {
	select {
	case p.loopKick <- false:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Reset runs the scheduled function and resets the scheduler
func (p *periodicCall) Reset(ctx context.Context) error {
	select {
	case p.loopKick <- true:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ResetTimer restarts the periodic schedule without running the scheduled
// function. It is best-effort and never blocks: if the loop is not running or
// is currently executing the scheduled function, the call is a no-op. Safe to
// call concurrently with the loop and before Run/after Close.
func (p *periodicCall) ResetTimer() {
	select {
	case p.loopReset <- struct{}{}:
	default:
	}
}

func (p *periodicCall) Close() {
	if !p.isRunning.Load() {
		return
	}
	p.loopCancel()
	<-p.loopDone
}

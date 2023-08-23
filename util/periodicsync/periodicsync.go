//go:generate mockgen -destination mock_periodicsync/mock_periodicsync.go github.com/anyproto/any-sync/util/periodicsync PeriodicSync
package periodicsync

import (
	"context"
	"github.com/anyproto/any-sync/app/logger"
	"go.uber.org/zap"
	"sync/atomic"
	"time"
)

type PeriodicSync interface {
	Run()
	Close()
}

type SyncerFunc func(ctx context.Context) error

func NewPeriodicSync(periodSeconds int, timeout time.Duration, caller SyncerFunc, l logger.CtxLogger) PeriodicSync {
	// TODO: rename to PeriodicCall (including folders) and do PRs in all repos where we are using this
	//  https://linear.app/anytype/issue/GO-1241/change-periodicsync-component-to-periodiccall
	ctx, cancel := context.WithCancel(context.Background())
	ctx = logger.CtxWithFields(ctx, zap.String("rootOp", "periodicCall"))
	return &periodicCall{
		caller:        caller,
		log:           l,
		loopCtx:       ctx,
		loopCancel:    cancel,
		loopDone:      make(chan struct{}),
		periodSeconds: periodSeconds,
		timeout:       timeout,
	}
}

type periodicCall struct {
	log           logger.CtxLogger
	caller        SyncerFunc
	loopCtx       context.Context
	loopCancel    context.CancelFunc
	loopDone      chan struct{}
	periodSeconds int
	timeout       time.Duration
	isRunning     atomic.Bool
}

func (p *periodicCall) Run() {
	p.isRunning.Store(true)
	go p.loop(p.periodSeconds)
}

func (p *periodicCall) loop(periodSeconds int) {
	period := time.Duration(periodSeconds) * time.Second
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
			case <-ticker.C:
				doCall()
			}
		}
	}
}

func (p *periodicCall) Close() {
	if !p.isRunning.Load() {
		return
	}
	p.loopCancel()
	<-p.loopDone
}

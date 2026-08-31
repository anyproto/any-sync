package ocache

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

type entryState int

const (
	entryStateLoading entryState = iota
	entryStateActive
	entryStateClosing
	entryStateClosed
)

type entry struct {
	id        string
	state     entryState
	lastUsage time.Time
	load      chan struct{}
	loadErr   error
	// loadAborted marks a failed load whose OWN context was already done
	// when the loadFunc returned — the load was killed (its first caller
	// went away, or the cache is closing), not refused by the loadFunc.
	// Written by oCache.load before the load channel closes; read only
	// after <-load, like loadErr.
	loadAborted bool
	value       Object
	close       chan struct{}
	mx          sync.Mutex
	// cancel aborts the load. Written once, before the entry is published
	// into oCache.data under c.mu; every cross-goroutine reader reaches the
	// entry through c.mu, which orders it after the write.
	cancel context.CancelFunc
}

func newEntry(id string, value Object, state entryState) *entry {
	return &entry{
		id:        id,
		load:      make(chan struct{}),
		lastUsage: time.Now(),
		state:     state,
		value:     value,
	}
}

// newLoadingEntry returns a loading entry with its load cancel registered.
// The cancel must exist before the entry becomes visible in the cache: a
// concurrent Close cancels loads through the entries it can see, and one it
// found without a cancel would run to completion uncancelled.
func newLoadingEntry(id string, ctx context.Context) (e *entry, loadCtx context.Context) {
	e = newEntry(id, nil, entryStateLoading)
	loadCtx, e.cancel = context.WithCancel(ctx)
	return
}

func (e *entry) isActive() bool {
	e.mx.Lock()
	defer e.mx.Unlock()
	return e.state == entryStateActive
}

func (e *entry) isClosing() bool {
	e.mx.Lock()
	defer e.mx.Unlock()
	return e.state == entryStateClosed || e.state == entryStateClosing
}

func (e *entry) cancelLoad() {
	e.mx.Lock()
	defer e.mx.Unlock()
	if e.cancel != nil {
		e.cancel()
	}
}

func (e *entry) waitLoad(ctx context.Context, id string) (value Object, err error) {
	select {
	case <-ctx.Done():
		// a completed load wins over a done ctx: Close passes a deadline
		// that may already be spent for entries late in its pass, and both
		// cases may have been ready with the select picking randomly —
		// re-check the load channel before failing. (The window is a few
		// instructions wide; no test can observe it, the guarantee is
		// structural.)
		select {
		case <-e.load:
			return e.value, e.loadErr
		default:
		}
		log.DebugCtx(ctx, "ctx done while waiting on object load", zap.String("id", id))
		return nil, ctx.Err()
	case <-e.load:
		return e.value, e.loadErr
	}
}

func (e *entry) waitClose(ctx context.Context, id string) (res bool, err error) {
	e.mx.Lock()
	switch e.state {
	case entryStateClosing:
		waitCh := e.close
		e.mx.Unlock()
		select {
		case <-ctx.Done():
			log.DebugCtx(ctx, "ctx done while waiting on object close", zap.String("id", id))
			return false, ctx.Err()
		case <-waitCh:
			return true, nil
		}
	case entryStateClosed:
		e.mx.Unlock()
		return true, nil
	default:
		e.mx.Unlock()
		return false, nil
	}
}

// setClosing transitions the entry to closing. With wait it blocks until another
// closer is done with it, bounded by ctx: that closer may be inside a TryClose
// that waits on an unresponsive peer. Acquisition is mode-dependent: without
// wait this call acquired the transition iff prevState == entryStateActive;
// with wait it may acquire the transition after waiting out another closer
// (prevState == entryStateClosing), so waiting callers must key on
// curState == entryStateClosing instead.
func (e *entry) setClosing(ctx context.Context, wait bool) (prevState, curState entryState, err error) {
	e.mx.Lock()
	prevState = e.state
	curState = e.state
	// A loading entry is refused: e.value is nil until oCache.load publishes
	// it, and e.load is closed by that load alone. Marking it closing would
	// hand the caller a nil value to close and would park any concurrent
	// waitClose waiter behind a close channel the load never closes (its
	// success path calls setActive(false)). Undoing the transition afterwards
	// is not equivalent: it either leaves the entry closing or makes it active
	// with a nil value. Callers that must close a loading entry (Remove,
	// RemoveSame, Close) wait the load out in oCache.removeCtx first.
	if e.state == entryStateLoading {
		e.mx.Unlock()
		return
	}
	// Loop rather than `if`: after waking from <-waitCh another goroutine may
	// have already moved the entry back to closing (e.g. a busy GC/TryRemove
	// reverted it to active and a concurrent remover re-acquired it). Re-check
	// and wait on the new close channel instead of overwriting e.close, which
	// would let two removers close the same channel twice.
	for e.state == entryStateClosing {
		waitCh := e.close
		e.mx.Unlock()
		if !wait {
			return
		}
		select {
		case <-waitCh:
		case <-ctx.Done():
			e.mx.Lock()
			curState = e.state
			e.mx.Unlock()
			return prevState, curState, ctx.Err()
		}
		e.mx.Lock()
	}
	if e.state != entryStateClosed {
		e.state = entryStateClosing
		e.close = make(chan struct{})
	}
	curState = e.state
	e.mx.Unlock()
	return
}

func (e *entry) setActive(chClose bool) {
	e.mx.Lock()
	defer e.mx.Unlock()
	// state before close: a close panic must not leave the entry in closing
	// with an already-closed channel, which would spin waitClose callers
	e.state = entryStateActive
	if chClose {
		close(e.close)
	}
}

func (e *entry) setClosed() {
	e.mx.Lock()
	defer e.mx.Unlock()
	e.state = entryStateClosed
	close(e.close)
}

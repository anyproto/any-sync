package ocache

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/logger"
)

var (
	ErrClosed    = errors.New("object cache closed")
	ErrExists    = errors.New("object exists")
	ErrNotExists = errors.New("object not exists")
	ErrNilValue  = errors.New("nil value")
)

// errLoadPanic marks an entry whose loadFunc panicked instead of returning.
var errLoadPanic = errors.New("load did not complete")

// isNilObject reports whether value is nil or a typed-nil pointer wrapped in
// the interface — calling methods on either panics in the close paths.
func isNilObject(value Object) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	}
	return false
}

var (
	defaultTTL = time.Minute
	defaultGC  = 20 * time.Second
	// bounds Close against entries the gc is already closing
	closeTimeout = 10 * time.Second
)

var log = logger.NewNamed("ocache")

type LoadFunc func(ctx context.Context, id string) (value Object, err error)

type Option func(*oCache)

var WithLogger = func(l *zap.SugaredLogger) Option {
	return func(cache *oCache) {
		cache.log = l
	}
}

var WithTTL = func(ttl time.Duration) Option {
	return func(cache *oCache) {
		cache.ttl = ttl
	}
}

var WithGCPeriod = func(gcPeriod time.Duration) Option {
	return func(cache *oCache) {
		cache.gc = gcPeriod
	}
}

// WithCloseTimeout bounds Close's waits on in-flight loads and on entries
// another closer holds (default 10s). The value.Close calls themselves take
// no ctx and are not bounded.
var WithCloseTimeout = func(d time.Duration) Option {
	return func(cache *oCache) {
		cache.closeTimeout = d
	}
}

func New(loadFunc LoadFunc, opts ...Option) OCache {
	c := &oCache{
		data:     make(map[string]*entry),
		loadFunc: loadFunc,
		timeNow:  time.Now,
		ttl:      defaultTTL,
		gc:       defaultGC,
		closeCh:  make(chan struct{}),
		log:      log.Sugar(),

		closeTimeout: closeTimeout,
	}
	for _, o := range opts {
		if o != nil {
			o(c)
		}
	}
	if c.ttl != 0 && c.gc != 0 {
		go c.ticker()
	}
	return c
}

type Object interface {
	Close() (err error)
	TryClose(objectTTL time.Duration) (res bool, err error)
}

type OCache interface {
	// DoLockedIfNotExists does an action if the object with id is not in cache
	// under a global lock, this will prevent a race which otherwise occurs
	// when object is created in parallel with action
	DoLockedIfNotExists(id string, action func() error) error
	// Get gets an object from cache or creates a new one via 'loadFunc';
	// it also refreshes the object's GC deadline.
	// When 'loadFunc' returns a non-nil error, an object will not be stored to cache.
	// A load that completed by the time ctx is done still returns its value.
	// Returns ErrClosed on a closed cache, including for waiters whose load
	// lands after Close.
	Get(ctx context.Context, id string) (value Object, err error)
	// Pick returns value if it's present in cache (will not call loadFunc,
	// does not refresh the GC deadline) — but it waits out an in-flight load
	// for the id, bounded by ctx. Returns ErrClosed on a closed cache.
	Pick(ctx context.Context, id string) (value Object, err error)
	// Add adds new object to cache
	// Returns error when the value is nil, the object exists or the cache is closed
	Add(id string, value Object) (err error)
	// Remove closes and removes object
	Remove(ctx context.Context, id string) (ok bool, err error)
	// RemoveSame closes and removes the object only if the value currently
	// stored under id is exactly the given one (pointer identity). It lets a
	// caller evict a specific instance it owns without racing a newer value
	// that has replaced it under the same id. Returns ok=true only when this
	// call performed the removal.
	RemoveSame(ctx context.Context, id string, value Object) (ok bool, err error)
	// TryRemove tries to close and to remove the object. ok reports whether
	// this call removed it; (false, nil) means the object declined to close,
	// is still loading, or another closer owns it. A non-nil err can
	// accompany ok=true when the value closed with an error.
	TryRemove(id string) (ok bool, err error)
	// ForEach iterates over all loaded objects, breaks when callback returns false
	ForEach(f func(v Object) (isContinue bool))
	// GC frees not used and expired objects
	// Will automatically called every 'gcPeriod'
	GC()
	// Len returns current cache size
	Len() int
	// Close closes all objects and the cache. The pass is bounded by a close
	// timeout; an entry held past it by an in-flight load or a busy closer is
	// not waited for — its value is closed by that path's own closed-cache
	// branch when it completes.
	Close() (err error)
}

type oCache struct {
	mu       sync.Mutex
	data     map[string]*entry
	loadFunc LoadFunc
	timeNow  func() time.Time
	ttl      time.Duration
	gc       time.Duration
	closed   bool
	closeCh  chan struct{}
	log      *zap.SugaredLogger
	metrics  *metrics

	closeTimeout time.Duration
}

// maxLoadRetries bounds Get's re-attempts after an aborted load (see the
// retry in Get). Each retry is a fresh load — the failed entry is deleted
// — so the bound only matters under a storm of loads that keep getting
// killed mid-flight.
const maxLoadRetries = 3

func (c *oCache) Get(ctx context.Context, id string) (value Object, err error) {
	var (
		counted bool
		retries int
	)
	for {
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return nil, ErrClosed
		}
		e, ok := c.data[id]
		load := false
		var loadCtx context.Context
		if !ok {
			e, loadCtx = newLoadingEntry(id, ctx)
			load = true
			c.data[id] = e
		}
		e.lastUsage = time.Now()
		c.mu.Unlock()
		reload, err := e.waitClose(ctx, id)
		if err != nil {
			return nil, err
		}
		if reload {
			continue
		}
		if !counted {
			c.metricsGet(!load)
			counted = true
		}
		if load {
			c.load(loadCtx, id, e)
			value, err = e.value, e.loadErr
		} else {
			value, err = e.waitLoad(ctx, id)
		}
		// A load runs under the context of whichever caller arrived first
		// and its result is shared with every concurrent waiter. If that
		// first caller goes away mid-load (its request finished, its
		// per-round timeout fired), the load is killed with an error that
		// has nothing to do with the waiters — retry instead of surfacing
		// it: the failed entry is already deleted, so the retry starts a
		// fresh load owned by a live context. loadAborted distinguishes a
		// killed load from a loadFunc that failed on its own (an internal
		// dial timeout returns context.DeadlineExceeded with the load's ctx
		// still alive — that is a verdict, not an abort, and is never
		// retried). The ctx.Err() check both scopes the retry to callers
		// that are still alive and proves waitLoad returned via the load
		// channel, making the loadAborted read safe.
		if err != nil && ctx.Err() == nil && e.loadAborted && retries < maxLoadRetries {
			retries++
			continue
		}
		return value, err
	}
}

func (c *oCache) Pick(ctx context.Context, id string) (value Object, err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, ErrClosed
	}
	val, ok := c.data[id]
	if !ok || val.isClosing() {
		c.mu.Unlock()
		return nil, ErrNotExists
	}
	c.mu.Unlock()
	c.metricsGet(true)
	return val.waitLoad(ctx, id)
}

// ctx is the cancellable load context Get created together with the entry.
func (c *oCache) load(ctx context.Context, id string, e *entry) {
	defer func() {
		// a panicking loadFunc arrives here with no result recorded; mark the
		// entry failed and drop it before releasing waiters, or the id wedges
		// forever with Get/Pick returning (nil, nil). The panic propagates to
		// the loading caller.
		if e.value == nil && e.loadErr == nil {
			c.mu.Lock()
			e.loadErr = errLoadPanic
			delete(c.data, id)
			c.mu.Unlock()
		}
		close(e.load)
	}()
	value, err := c.loadFunc(ctx, id)
	// Read before cancelLoad(): a done ctx here means the load was killed
	// (first caller gone, or cancelLoad on cache close) rather than the
	// loadFunc failing on its own — recorded so Get's waiters can retry.
	aborted := ctx.Err() != nil
	e.cancelLoad()

	c.mu.Lock()
	if isNilObject(value) && err == nil {
		err = fmt.Errorf("loaded %w, id: %s", ErrNilValue, id)
	}
	if err != nil {
		e.loadErr = err
		e.loadAborted = aborted
		delete(c.data, id)
		c.mu.Unlock()
		return
	}
	if c.closed {
		// Close has already run (and possibly given up waiting on this load):
		// republishing the value into a closed cache would leak it, since no
		// path closes entries after Close. Close it here instead; waiters get
		// ErrClosed. The deferred close(e.load) fires after value.Close(), so
		// anyone woken by it observes the value already closed.
		e.loadErr = ErrClosed
		delete(c.data, id)
		c.mu.Unlock()
		if cErr := value.Close(); cErr != nil {
			c.log.With("object_id", id).Warnf("load after close: value close error: %v", cErr)
		}
		return
	}
	e.value = value
	e.setActive(false)
	c.mu.Unlock()
}

func (c *oCache) Remove(ctx context.Context, id string) (ok bool, err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		err = ErrClosed
		return
	}
	e, ok := c.data[id]
	if !ok {
		c.mu.Unlock()
		return false, ErrNotExists
	}
	c.mu.Unlock()
	return c.remove(ctx, e)
}

// closeAndDelete finalizes a closing entry under c.mu. The deferred unlock
// guarantees c.mu is released even if setClosed panics, so a panic in the
// close path can never leave the whole cache wedged (GO-7332 hardening).
func (c *oCache) closeAndDelete(e *entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e.setClosed()
	delete(c.data, e.id)
}

func (c *oCache) remove(ctx context.Context, e *entry) (ok bool, err error) {
	return c.removeCtx(ctx, ctx, e)
}

// loadCtx bounds waiting for an in-flight load, closingCtx bounds waiting for
// another closer to release the entry
func (c *oCache) removeCtx(loadCtx, closingCtx context.Context, e *entry) (ok bool, err error) {
	if _, err = e.waitLoad(loadCtx, e.id); err != nil {
		return false, err
	}
	_, curState, err := e.setClosing(closingCtx, true)
	if err != nil {
		return false, err
	}
	if curState == entryStateClosing {
		ok = true
		err = e.value.Close()
		c.closeAndDelete(e)
	}
	return
}

func (c *oCache) RemoveSame(ctx context.Context, id string, value Object) (ok bool, err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return false, ErrClosed
	}
	e, exists := c.data[id]
	// e.value is written only under c.mu (in Add/load), so reading it here is
	// race-free. A nil value never matches: a mid-load entry's value is still
	// nil, and matching it would wait the load out and close whatever object
	// it publishes. remove() acts on this exact entry: it deletes/closes it only
	// if this call is the one that transitions it to closing. If e was already
	// replaced under the same id it is in a closed state and remove() is a
	// no-op, so a stale caller can never close the newer value that took the id.
	same := exists && value != nil && e.value == value
	c.mu.Unlock()
	if !same {
		return false, ErrNotExists
	}
	return c.remove(ctx, e)
}

func (c *oCache) TryRemove(id string) (ok bool, err error) {
	c.mu.Lock()

	if c.closed {
		c.mu.Unlock()
		return false, ErrClosed
	}

	e, contains := c.data[id]
	if !contains {
		c.mu.Unlock()
		return false, ErrNotExists
	}

	c.mu.Unlock()

	ok, err = c.tryCloseEntry(e)
	if err != nil {
		c.log.With("object_id", e.id).Warnf("try remove err: %v", err)
	}
	return ok, err
}

// tryCloseEntry acquires the closing transition and asks the value whether it
// can close. Only prevState == entryStateActive means this call acquired the
// transition and may touch e.value; loading refuses it, closing/closed belong
// to another closer. A value that declined or errored is restored to active —
// abandoning an acquired transition would park the entry in closing behind a
// close channel nobody ever closes, wedging the id. A closed value is
// finalized even when TryClose also returned an error.
func (c *oCache) tryCloseEntry(e *entry) (closed bool, err error) {
	prevState, _, _ := e.setClosing(context.Background(), false)
	if prevState != entryStateActive {
		return false, nil
	}
	closed, err = e.value.TryClose(c.ttl)
	if !closed {
		c.mu.Lock()
		cacheClosed := c.closed
		c.mu.Unlock()
		if cacheClosed {
			// Close's pass may already have given up waiting on this entry:
			// restoring it to active would resurrect a live value into a
			// closed cache with no remaining closer. Escalate the decline
			// instead — the mirror of load's closed-cache branch. If Close is
			// still parked on e.close, setClosed wakes it into a safe refusal.
			cErr := e.value.Close()
			if err == nil {
				err = cErr
			}
			c.closeAndDelete(e)
			return true, err
		}
		e.setActive(true)
		return false, err
	}
	c.closeAndDelete(e)
	return true, err
}

func (c *oCache) DoLockedIfNotExists(id string, action func() error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrClosed
	}
	if _, ok := c.data[id]; ok {
		return ErrExists
	}
	return action()
}

func (c *oCache) Add(id string, value Object) (err error) {
	// a nil value would panic every close path; the GC one runs on the
	// unrecovered ticker goroutine and would take the process down
	if isNilObject(value) {
		return ErrNilValue
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrClosed
	}
	if _, ok := c.data[id]; ok {
		return ErrExists
	}
	e := newEntry(id, value, entryStateActive)
	close(e.load)
	c.data[id] = e
	return
}

func (c *oCache) ForEach(f func(obj Object) (isContinue bool)) {
	var objects []Object
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	for _, v := range c.data {
		select {
		case <-v.load:
			if v.value != nil && !v.isClosing() {
				objects = append(objects, v.value)
			}
		default:
		}
	}
	c.mu.Unlock()
	for _, obj := range objects {
		if !f(obj) {
			return
		}
	}
}

func (c *oCache) ticker() {
	ticker := time.NewTicker(c.gc)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.GC()
		case <-c.closeCh:
			return
		}
	}
}

func (c *oCache) GC() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	deadline := c.timeNow().Add(-c.ttl)
	var toClose []*entry
	for _, e := range c.data {
		if e.isActive() && e.lastUsage.Before(deadline) {
			toClose = append(toClose, e)
		}
	}
	size := len(c.data)
	c.mu.Unlock()
	closedNum := 0
	for _, e := range toClose {
		closed, err := c.tryCloseEntry(e)
		if err != nil {
			c.log.With("object_id", e.id).Warnf("GC: object close error: %v", err)
		}
		if closed {
			closedNum++
		}
	}
	c.metricsClosed(closedNum, size)
}

func (c *oCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.data)
}

func (c *oCache) Close() (err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrClosed
	}
	c.closed = true
	close(c.closeCh)
	var toClose []*entry
	for _, e := range c.data {
		e.cancelLoad()
		toClose = append(toClose, e)
	}
	c.mu.Unlock()
	// one deadline for the whole pass, spent only on entries another closer or
	// an unresponsive load holds: that closer can be a gc stuck in TryClose on
	// an unresponsive peer, and a loadFunc that ignores its cancelled ctx must
	// forfeit its entry rather than wedge Close — its value is closed by
	// c.load itself when it finally completes (the c.closed branch there).
	// Completed loads win over the spent deadline inside waitLoad, and
	// value.Close takes no ctx, so uncontended entries still close normally
	// once the deadline has passed. ErrClosed from waitLoad means c.load
	// already handled the entry, so it is not worth a warning.
	closingCtx, cancel := context.WithTimeout(context.Background(), c.closeTimeout)
	defer cancel()
	for _, e := range toClose {
		// ErrClosed means c.load already handled the entry; context.Canceled
		// is the loadErr of a load this Close killed itself — neither is
		// worth a warning
		if _, err := c.removeCtx(closingCtx, closingCtx, e); err != nil &&
			!errors.Is(err, ErrNotExists) && !errors.Is(err, ErrClosed) && !errors.Is(err, context.Canceled) {
			c.log.With("object_id", e.id).Warnf("cache close: object close error: %v", err)
		}
	}
	return nil
}

func (c *oCache) metricsGet(hit bool) {
	if c.metrics == nil {
		return
	}
	if hit {
		c.metrics.hit.Inc()
	} else {
		c.metrics.miss.Inc()
	}
}

func (c *oCache) metricsClosed(closedLen, size int) {
	c.log.Infof("GC: removed %d; cache size: %d", closedLen, size)
	if c.metrics == nil || closedLen == 0 {
		return
	}
	c.metrics.gc.Add(float64(closedLen))
}

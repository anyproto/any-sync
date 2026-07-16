package ocache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/logger"
)

var (
	ErrClosed    = errors.New("object cache closed")
	ErrExists    = errors.New("object exists")
	ErrNotExists = errors.New("object not exists")
)

var (
	defaultTTL = time.Minute
	defaultGC  = 20 * time.Second
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

func New(loadFunc LoadFunc, opts ...Option) OCache {
	c := &oCache{
		data:     make(map[string]*entry),
		loadFunc: loadFunc,
		timeNow:  time.Now,
		ttl:      defaultTTL,
		gc:       defaultGC,
		closeCh:  make(chan struct{}),
		log:      log.Sugar(),
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
	// Get gets an object from cache or create a new one via 'loadFunc'
	// Increases the object refs counter on successful
	// When 'loadFunc' returns a non-nil error, an object will not be stored to cache
	Get(ctx context.Context, id string) (value Object, err error)
	// Pick returns value if it's presents in cache (will not call loadFunc)
	Pick(ctx context.Context, id string) (value Object, err error)
	// Add adds new object to cache
	// Returns error when object exists
	Add(id string, value Object) (err error)
	// Remove closes and removes object
	Remove(ctx context.Context, id string) (ok bool, err error)
	// RemoveSame closes and removes the object only if the value currently
	// stored under id is exactly the given one (pointer identity). It lets a
	// caller evict a specific instance it owns without racing a newer value
	// that has replaced it under the same id. Returns ok=true only when this
	// call performed the removal.
	RemoveSame(ctx context.Context, id string, value Object) (ok bool, err error)
	// TryRemove Tries to close and to remove the object
	TryRemove(id string) (ok bool, err error)
	// ForEach iterates over all loaded objects, breaks when callback returns false
	ForEach(f func(v Object) (isContinue bool))
	// GC frees not used and expired objects
	// Will automatically called every 'gcPeriod'
	GC()
	// Len returns current cache size
	Len() int
	// Close closes all objects and cache
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
		if !ok {
			e = newEntry(id, nil, entryStateLoading)
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
			c.load(ctx, id, e)
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
	val, ok := c.data[id]
	if !ok || val.isClosing() {
		c.mu.Unlock()
		return nil, ErrNotExists
	}
	c.mu.Unlock()
	c.metricsGet(true)
	return val.waitLoad(ctx, id)
}

func (c *oCache) load(ctx context.Context, id string, e *entry) {
	defer close(e.load)
	ctx, cancel := context.WithCancel(ctx)
	e.setCancel(cancel)
	value, err := c.loadFunc(ctx, id)
	// Read before cancel(): a done ctx here means the load was killed
	// (first caller gone, or cancelLoad on cache close) rather than the
	// loadFunc failing on its own — recorded so Get's waiters can retry.
	aborted := ctx.Err() != nil
	cancel()

	c.mu.Lock()
	defer c.mu.Unlock()
	if value == nil && err == nil {
		err = fmt.Errorf("loaded value is nil, id: %s", id)
	}
	if err != nil {
		e.loadErr = err
		e.loadAborted = aborted
		delete(c.data, id)
	} else {
		e.value = value
		e.setActive(false)
	}
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
	if _, err = e.waitLoad(ctx, e.id); err != nil {
		return false, err
	}
	_, curState := e.setClosing(true)
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
	// race-free. remove() acts on this exact entry: it deletes/closes it only
	// if this call is the one that transitions it to closing. If e was already
	// replaced under the same id it is in a closed state and remove() is a
	// no-op, so a stale caller can never close the newer value that took the id.
	same := exists && e.value == value
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

	prevState, _ := e.setClosing(false)
	if prevState == entryStateClosing || prevState == entryStateClosed {
		return false, nil
	}

	closed, err := e.value.TryClose(c.ttl)
	if err != nil {
		c.log.With("object_id", e.id).Warnf("try remove err: %v", err)
		return closed, err
	}

	if !closed {
		e.setActive(true)
		return false, nil
	}

	c.closeAndDelete(e)
	return true, nil
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
	c.mu.Lock()
	defer c.mu.Unlock()
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
		prevState, _ := e.setClosing(false)
		if prevState == entryStateClosing || prevState == entryStateClosed {
			continue
		}
		closed, err := e.value.TryClose(c.ttl)
		if err != nil {
			c.log.With("object_id", e.id).Warnf("GC: object close error: %v", err)
		}
		if !closed {
			e.setActive(true)
			continue
		} else {
			closedNum++
			c.closeAndDelete(e)
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
	for _, e := range toClose {
		if _, err := c.remove(context.Background(), e); err != nil && err != ErrNotExists {
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

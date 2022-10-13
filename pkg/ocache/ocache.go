package ocache

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"go.uber.org/zap"
	"sync"
	"time"
)

var (
	ErrClosed    = errors.New("object cache closed")
	ErrExists    = errors.New("object exists")
	ErrTimeout   = errors.New("loading object timed out")
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

var WithRefCounter = func(enable bool) Option {
	return func(cache *oCache) {
		cache.noRefCounter = !enable
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
		o(c)
	}
	if c.ttl != 0 {
		go c.ticker()
	}
	return c
}

type Object interface {
	Close() (err error)
}

type ObjectLocker interface {
	Locked() bool
}

type ObjectLastUsage interface {
	LastUsage() time.Time
}

type entry struct {
	id        string
	lastUsage time.Time
	refCount  uint32
	load      chan struct{}
	loadErr   error
	value     Object
	isClosing bool
	close     chan struct{}
}

func (e *entry) locked() bool {
	if locker, ok := e.value.(ObjectLocker); ok {
		return locker.Locked()
	}
	return false
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
	// Release decreases the refs counter
	Release(id string) bool
	// Reset sets refs counter to 0
	Reset(id string) bool
	// Remove closes and removes object
	Remove(id string) (ok bool, err error)
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
	mu           sync.Mutex
	data         map[string]*entry
	loadFunc     LoadFunc
	timeNow      func() time.Time
	ttl          time.Duration
	gc           time.Duration
	closed       bool
	closeCh      chan struct{}
	log          *zap.SugaredLogger
	noRefCounter bool
}

func (c *oCache) Get(ctx context.Context, id string) (value Object, err error) {
	var (
		e    *entry
		ok   bool
		load bool
	)
Load:
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, ErrClosed
	}
	if e, ok = c.data[id]; !ok {
		load = true
		e = &entry{
			id:   id,
			load: make(chan struct{}),
		}
		c.data[id] = e
	}
	closing := e.isClosing
	if !e.isClosing {
		e.lastUsage = c.timeNow()
		if !c.noRefCounter {
			e.refCount++
		}
	}
	c.mu.Unlock()
	if closing {
		<-e.close
		goto Load
	}

	if load {
		go c.load(ctx, id, e)
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-e.load:
	}
	return e.value, e.loadErr
}

func (c *oCache) Pick(ctx context.Context, id string) (value Object, err error) {
	c.mu.Lock()
	val, ok := c.data[id]
	if !ok || val.isClosing {
		c.mu.Unlock()
		return nil, ErrNotExists
	}
	c.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-val.load:
		return val.value, val.loadErr
	}
}

func (c *oCache) load(ctx context.Context, id string, e *entry) {
	defer close(e.load)
	value, err := c.loadFunc(ctx, id)

	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		e.loadErr = err
		delete(c.data, id)
	} else {
		e.value = value
	}
}

func (c *oCache) Release(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	if e, ok := c.data[id]; ok {
		if !c.noRefCounter && e.refCount > 0 {
			e.refCount--
			return true
		}
	}
	return false
}

func (c *oCache) Reset(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	if e, ok := c.data[id]; ok {
		e.refCount = 0
		return true
	}
	return false
}

func (c *oCache) Remove(id string) (ok bool, err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		err = ErrClosed
		return
	}
	var e *entry
	e, ok = c.data[id]
	if !ok || e.isClosing {
		c.mu.Unlock()
		return
	}
	e.isClosing = true
	e.close = make(chan struct{})
	c.mu.Unlock()

	<-e.load
	if e.value != nil {
		err = e.value.Close()
	}
	c.mu.Lock()
	close(e.close)
	delete(c.data, e.id)
	c.mu.Unlock()

	return
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
	e := &entry{
		id:        id,
		lastUsage: time.Now(),
		refCount:  0,
		load:      make(chan struct{}),
		value:     value,
	}
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
			if v.value != nil && !v.isClosing {
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
		if e.isClosing {
			continue
		}
		lu := e.lastUsage
		if lug, ok := e.value.(ObjectLastUsage); ok {
			lu = lug.LastUsage()
		}
		if !e.locked() && e.refCount <= 0 && lu.Before(deadline) {
			e.isClosing = true
			e.close = make(chan struct{})
			toClose = append(toClose, e)
		}
	}
	size := len(c.data)
	c.mu.Unlock()

	c.log.Infof("GC: removed %d; cache size: %d", len(toClose), size)
	for _, e := range toClose {
		<-e.load
		if e.value != nil {
			if err := e.value.Close(); err != nil {
				c.log.With("object_id", e.id).Warnf("GC: object close error: %v", err)
			}
		}
	}

	c.mu.Lock()
	for _, e := range toClose {
		close(e.close)
		delete(c.data, e.id)
	}
	c.mu.Unlock()
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
		toClose = append(toClose, e)
	}
	c.mu.Unlock()
	for _, e := range toClose {
		<-e.load
		if e.value != nil {
			if clErr := e.value.Close(); clErr != nil {
				c.log.With("object_id", e.id).Warnf("cache close: object close error: %v", clErr)
			}
		}
	}
	return nil
}

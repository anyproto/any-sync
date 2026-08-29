package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/logger"
)

var (
	// values of this vars will be defined while compilation
	AppName, GitCommit, GitBranch, GitState, GitSummary, BuildDate string
	name                                                           string
)

var (
	log                  = logger.NewNamed("app")
	StopDeadline         = time.Minute
	StopWarningAfter     = time.Second * 10
	StartWarningAfter    = time.Second * 10
	ErrComponentNotFound = errors.New("component not found")
	// ErrAppAlreadyStarted is returned by a second Start on the same App.
	ErrAppAlreadyStarted = errors.New("app already started")
	// ErrAppClosed is returned by Start on an App that has been closed.
	ErrAppClosed = errors.New("app closed")
)

// Component is a minimal interface for a common app.Component
type Component interface {
	// Init will be called first
	// When returned error is not nil - app start will be aborted
	Init(a *App) (err error)
	// Name must return unique service name
	Name() (name string)
}

// ComponentRunnable is an interface for realizing ability to start background processes or deep configure service
type ComponentRunnable interface {
	Component
	// Run will be called after init stage
	// Non-nil error also will be aborted app start
	Run(ctx context.Context) (err error)
	// Close will be called when app shutting down
	// Also will be called when service return error on Init or Run stage
	// Non-nil error will be printed to log
	Close(ctx context.Context) (err error)
}

type ComponentStatable interface {
	StateChange(state int)
}

// App is the central part of the application
// It contains and manages all components
type App struct {
	parent      *App
	components  []Component
	mu          sync.RWMutex
	startStat   Stat
	stopStat    Stat
	deviceState int
	// lifecycleMu makes Start and Close mutually exclusive and guards the
	// four fields below. It is NOT mu: a component's Init calls
	// MustComponent, which takes mu.RLock, so Start can never hold mu's
	// write lock.
	lifecycleMu sync.Mutex
	started     bool
	closed      bool
	// initialized is the Init high-water mark: components [0, initialized)
	// had Init reached. closedUpTo is how far Start's own cleanup already
	// closed, so Close covers exactly [closedUpTo, initialized).
	initialized       int
	closedUpTo        int
	versionName       string
	anySyncVersion    string
	componentListener func(comp Component)
}

// Name returns app name
func (app *App) Name() string {
	return name
}

func (app *App) AppName() string {
	return AppName
}

// Version return app version
func (app *App) Version() string {
	return GitSummary
}

// SetVersionName sets the custom application version
func (app *App) SetVersionName(v string) {
	app.versionName = v
}

// VersionName returns a string with the settled app version or auto-generated version if it didn't set
func (app *App) VersionName() string {
	if app.versionName != "" {
		return app.versionName
	}
	return AppName + ":" + GitSummary + "/any-sync:" + app.AnySyncVersion()
}

type Stat struct {
	SpentMsPerComp map[string]int64
	SpentMsTotal   int64
}

// StartStat returns total time spent per comp for the last Start
func (app *App) StartStat() Stat {
	app.mu.Lock()
	defer app.mu.Unlock()
	return app.startStat
}

// StopStat returns total time spent per comp for the last Close
func (app *App) StopStat() Stat {
	app.mu.Lock()
	defer app.mu.Unlock()
	return app.stopStat
}

// VersionDescription return the full info about the build
func (app *App) VersionDescription() string {
	return VersionDescription()
}

func Version() string {
	return GitSummary
}

func VersionDescription() string {
	return fmt.Sprintf("build on %s from %s at #%s(%s)", BuildDate, GitBranch, GitCommit, GitState)
}

// ChildApp creates a child container which has access to parent's components
// It doesn't call Start on any of the parent's components
func (app *App) ChildApp() *App {
	return &App{
		parent:            app,
		deviceState:       app.deviceState,
		anySyncVersion:    app.AnySyncVersion(),
		componentListener: app.componentListener,
	}
}

// Register adds service to registry
// All components will be started in the order they were registered
func (app *App) Register(s Component) *App {
	app.mu.Lock()
	defer app.mu.Unlock()
	for _, es := range app.components {
		if s.Name() == es.Name() {
			panic(fmt.Errorf("component '%s' already registered", s.Name()))
		}
	}
	app.components = append(app.components, s)
	return app
}

// Component returns service by name
// If service with given name wasn't registered, nil will be returned
func (app *App) Component(name string) Component {
	app.mu.RLock()
	defer app.mu.RUnlock()
	current := app
	for current != nil {
		for _, s := range current.components {
			if s.Name() == name {
				app.onComponent(s)
				return s
			}
		}
		current = current.parent
	}
	return nil
}

// MustComponent is like Component, but it will panic if service wasn't found
func (app *App) MustComponent(name string) Component {
	s := app.Component(name)
	if s == nil {
		panic(fmt.Errorf("component '%s' not registered", name))
	}
	return s
}

func GetComponent[t any](app *App) (t, error) {
	app.mu.RLock()
	defer app.mu.RUnlock()
	var empty t
	current := app
	for current != nil {
		for _, s := range current.components {
			if v, ok := s.(t); ok {
				app.onComponent(s)
				return v, nil
			}
		}
		current = current.parent
	}
	return empty, ErrComponentNotFound
}

// MustComponent - generic version of app.MustComponent
func MustComponent[t any](app *App) t {
	component, err := GetComponent[t](app)
	if err != nil {
		panic(fmt.Errorf("component with interface %T is not found", new(t)))
	}
	return component
}

// ComponentNames returns all registered names
func (app *App) ComponentNames() (names []string) {
	app.mu.RLock()
	defer app.mu.RUnlock()
	names = make([]string, 0, len(app.components))
	current := app
	for current != nil {
		for _, c := range current.components {
			names = append(names, c.Name())
		}
		current = current.parent
	}
	return
}

// Start starts the application
// All registered services will be initialized and started
//
// Start runs at most once per App: a second call returns ErrAppAlreadyStarted,
// and an App that has already been closed returns ErrAppClosed. Start and
// Close are mutually exclusive, so a component is never closed while its Init
// is still running; a Close racing a Start waits for it.
func (app *App) Start(ctx context.Context) (err error) {
	app.lifecycleMu.Lock()
	defer app.lifecycleMu.Unlock()
	if app.started {
		return ErrAppAlreadyStarted
	}
	if app.closed {
		return ErrAppClosed
	}
	app.started = true

	app.mu.RLock()
	defer app.mu.RUnlock()
	app.startStat.SpentMsPerComp = make(map[string]int64)
	var currentComponentStarting string
	done := make(chan struct{})
	// Every return path stops the watchdog: otherwise a failed Start leaks a
	// goroutine and a timer, and that goroutine reads stopStat's map while a
	// concurrent Close writes it.
	defer close(done)
	go func() {
		select {
		case <-done:
			return
		case <-time.After(StartWarningAfter):
			l := statLogger(app.startStat, log).With(zap.String("in_progress", currentComponentStarting))
			l.Warn("components start in progress")
		}
	}()
	closeServices := func(idx int) {
		for i := idx; i >= 0; i-- {
			if serviceClose, ok := app.components[i].(ComponentRunnable); ok {
				if e := serviceClose.Close(ctx); e != nil {
					log.Error("close error", zap.String("component", serviceClose.Name()), zap.Error(e))
				}
			}
		}
		app.closedUpTo = idx + 1
	}

	for i, s := range app.components {
		currentComponentStarting = s.Name()
		if err = s.Init(app); err != nil {
			log.Error("can't init service", zap.String("service", s.Name()), zap.Error(err))
			// The failing component is closed too (documented), but nothing
			// past it ever ran Init — its fields are nil.
			app.initialized = i + 1
			closeServices(i)
			return fmt.Errorf("can't init service '%s': %w", s.Name(), err)
		}
		// Advance only once Init has returned: Close must never see a
		// component that is still initialising.
		app.initialized = i + 1
	}

	for i, s := range app.components {
		if serviceRun, ok := s.(ComponentRunnable); ok {
			currentComponentStarting = s.Name()
			start := time.Now()
			if err = serviceRun.Run(ctx); err != nil {
				log.Error("can't run service", zap.String("service", serviceRun.Name()), zap.Error(err))
				closeServices(i)
				return fmt.Errorf("can't run service '%s': %w", serviceRun.Name(), err)
			}
			spent := time.Since(start).Milliseconds()
			app.startStat.SpentMsTotal += spent
			app.startStat.SpentMsPerComp[s.Name()] = spent
		}
	}

	l := statLogger(app.startStat, log)
	if app.startStat.SpentMsTotal > StartWarningAfter.Milliseconds() {
		l.Warn("all components started")
	}
	l.Debug("all components started")
	return
}

// IterateComponents iterates over all registered components. It's safe for concurrent use.
func (app *App) IterateComponents(fn func(Component)) {
	app.mu.RLock()
	defer app.mu.RUnlock()
	for _, s := range app.components {
		fn(s)
	}
}

func stackAllGoroutines() []byte {
	for size := 4096 * 1024; ; size *= 2 {
		buf := make([]byte, size)
		if n := runtime.Stack(buf, true); n < size {
			return buf[:n]
		}
	}
}

func statLogger(stat Stat, ctxLogger logger.CtxLogger) logger.CtxLogger {
	l := ctxLogger
	for k, v := range stat.SpentMsPerComp {
		l = l.With(zap.Int64(k, v))
	}
	l = l.With(zap.Int64("total", stat.SpentMsTotal))

	return l
}

// Close stops the application
//
// Components with a ComponentRunnable implementation are closed in reverse
// registration order, but only those whose Init was reached: a component past
// a failed Init has nil fields and closing it would panic. Anything Start's
// own error path already closed is not closed again. Close is idempotent —
// a second call is a no-op returning nil — and waits for an in-flight Start.
func (app *App) Close(ctx context.Context) error {
	log.Debug("close components...")
	app.lifecycleMu.Lock()
	defer app.lifecycleMu.Unlock()
	if app.closed {
		return nil
	}
	app.closed = true

	app.mu.RLock()
	defer app.mu.RUnlock()
	app.stopStat.SpentMsPerComp = make(map[string]int64)
	var currentComponentStopping string
	done := make(chan struct{})

	go func() {
		select {
		case <-done:
			return
		case <-time.After(StopWarningAfter):
			statLogger(app.stopStat, log).
				With(zap.String("in_progress", currentComponentStopping)).
				Warn("components close in progress")
		}
	}()
	go func() {
		select {
		case <-done:
			return
		case <-time.After(StopDeadline):
			_, _ = os.Stderr.Write([]byte("app.Close timeout\n"))
			_, _ = os.Stderr.Write(stackAllGoroutines())
			panic("app.Close timeout")
		}
	}()

	var errs []string
	for i := app.initialized - 1; i >= app.closedUpTo; i-- {
		if serviceClose, ok := app.components[i].(ComponentRunnable); ok {
			start := time.Now()
			currentComponentStopping = app.components[i].Name()
			if e := serviceClose.Close(ctx); e != nil {
				errs = append(errs, fmt.Sprintf("Component '%s' close error: %v", serviceClose.Name(), e))
			}
			spent := time.Since(start).Milliseconds()
			app.stopStat.SpentMsTotal += spent
			app.stopStat.SpentMsPerComp[app.components[i].Name()] = spent
		}
	}
	close(done)
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	l := statLogger(app.stopStat, log)
	if app.stopStat.SpentMsTotal > StopWarningAfter.Milliseconds() {
		l.Warn("all components have been closed")
	}

	l.Debug("all components have been closed")
	return nil
}

func (app *App) SetDeviceState(state int) {
	if app == nil {
		return
	}
	app.mu.RLock()
	defer app.mu.RUnlock()
	app.deviceState = state
	for _, component := range app.components {
		if statable, ok := component.(ComponentStatable); ok {
			statable.StateChange(state)
		}
	}
}

var onceVersion sync.Once

func (app *App) AnySyncVersion() string {
	onceVersion.Do(func() {
		info, ok := debug.ReadBuildInfo()
		if ok {
			for _, mod := range info.Deps {
				if mod.Path == "github.com/anyproto/any-sync" {
					app.anySyncVersion = mod.Version
					break
				}
			}
		}
	})
	return app.anySyncVersion
}

func (app *App) onComponent(s Component) {
	if app.componentListener != nil {
		app.componentListener(s)
	}
}

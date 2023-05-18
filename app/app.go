package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/any-sync/app/logger"
	"go.uber.org/zap"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

var (
	// values of this vars will be defined while compilation
	AppName, GitCommit, GitBranch, GitState, GitSummary, BuildDate string
	name                                                           string
)

var (
	log               = logger.NewNamed("app")
	StopDeadline      = time.Minute
	StopWarningAfter  = time.Second * 10
	StartWarningAfter = time.Second * 10
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
	components     []Component
	mu             sync.RWMutex
	startStat      Stat
	stopStat       Stat
	deviceState    int
	anySyncVersion string
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
	for _, s := range app.components {
		if s.Name() == name {
			return s
		}
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

// MustComponent - generic version of app.MustComponent
func MustComponent[i any](app *App) i {
	app.mu.RLock()
	defer app.mu.RUnlock()
	for _, s := range app.components {
		if v, ok := s.(i); ok {
			return v
		}
	}
	empty := new(i)
	panic(fmt.Errorf("component with interface %T is not found", empty))
}

// ComponentNames returns all registered names
func (app *App) ComponentNames() (names []string) {
	app.mu.RLock()
	defer app.mu.RUnlock()
	names = make([]string, len(app.components))
	for i, c := range app.components {
		names[i] = c.Name()
	}
	return
}

// Start starts the application
// All registered services will be initialized and started
func (app *App) Start(ctx context.Context) (err error) {
	app.mu.RLock()
	defer app.mu.RUnlock()
	app.startStat.SpentMsPerComp = make(map[string]int64)
	var currentComponentStarting string
	done := make(chan struct{})
	go func() {
		select {
		case <-done:
			return
		case <-time.After(StartWarningAfter):
			l := statLogger(app.stopStat, log).With(zap.String("in_progress", currentComponentStarting))
			l.Warn("components start in progress")
		}
	}()
	closeServices := func(idx int) {
		for i := idx; i >= 0; i-- {
			if serviceClose, ok := app.components[i].(ComponentRunnable); ok {
				if e := serviceClose.Close(ctx); e != nil {
					log.Info("close error", zap.String("component", serviceClose.Name()), zap.Error(e))
				}
			}
		}
	}

	for i, s := range app.components {
		if err = s.Init(app); err != nil {
			closeServices(i)
			return fmt.Errorf("can't init service '%s': %w", s.Name(), err)
		}
	}

	for i, s := range app.components {
		if serviceRun, ok := s.(ComponentRunnable); ok {
			start := time.Now()
			if err = serviceRun.Run(ctx); err != nil {
				closeServices(i)
				return fmt.Errorf("can't run service '%s': %w", serviceRun.Name(), err)
			}
			spent := time.Since(start).Milliseconds()
			app.startStat.SpentMsTotal += spent
			app.startStat.SpentMsPerComp[s.Name()] = spent
		}
	}

	close(done)
	l := statLogger(app.stopStat, log)
	if app.startStat.SpentMsTotal > StartWarningAfter.Milliseconds() {
		l.Warn("all components started")
	}
	l.Debug("all components started")
	return
}

func stackAllGoroutines() []byte {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return buf[:n]
		}
		buf = make([]byte, 2*len(buf))
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
// All components with ComponentRunnable implementation will be closed in the reversed order
func (app *App) Close(ctx context.Context) error {
	log.Debug("close components...")
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
	for i := len(app.components) - 1; i >= 0; i-- {
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
		fmt.Println("111")
		info, ok := debug.ReadBuildInfo()
		if ok {
			for _, mod := range info.Deps {
				if mod.Path == "github.com/anytypeio/any-sync" {
					app.anySyncVersion = mod.Version
					break
				}
			}
		}
	})
	return app.anySyncVersion
}

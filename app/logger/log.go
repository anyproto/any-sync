package logger

import (
	"sync"

	"github.com/gobwas/glob"
	"go.uber.org/zap"
)

var (
	mu                sync.Mutex
	logger            *zap.Logger
	loggerConfig      zap.Config
	namedLevels       []namedLevel
	namedGlobs        = make(map[string]glob.Glob)
	namedLoggers      = make(map[string]CtxLogger)
	namedSugarLoggers = make(map[string]*zap.SugaredLogger)
)

type namedLevel struct {
	name  string
	level zap.AtomicLevel
}

func init() {
	loggerConfig = zap.NewDevelopmentConfig()
	logger, _ = loggerConfig.Build()
}

// SetDefault replaces the default logger
// you need to call SetNamedLevels after in case you have named loggers,
// otherwise they will use the old logger
func SetDefault(l *zap.Logger) {
	mu.Lock()
	defer mu.Unlock()
	*logger = *l
}

// SetNamedLevels sets the namedLevels for named loggers
// it also supports glob patterns for names, like "app*"
// can be racy in case there are existing named loggers
// so consider to call only once at the beginning
func SetNamedLevels(nls []NamedLevel) {
	mu.Lock()
	defer mu.Unlock()
	namedLevels = namedLevels[:0]

	var minLevel = logger.Level()
	for _, nl := range nls {
		l, err := zap.ParseAtomicLevel(nl.Level)
		if err != nil {
			continue
		}
		namedLevels = append(namedLevels, namedLevel{name: nl.Name, level: l})
		g, err := glob.Compile(nl.Name)
		if err == nil {
			namedGlobs[nl.Name] = g
		}

		if l.Level() < minLevel {
			minLevel = l.Level()
		}
	}

	if minLevel < logger.Level() {
		// recreate logger if the min level is lower than the current min one
		loggerConfig.Level = zap.NewAtomicLevelAt(minLevel)
		logger, _ = loggerConfig.Build()
	}

	for name, nl := range namedLoggers {
		level := getLevel(name)
		newCore := zap.New(logger.Core()).Named(name).WithOptions(
			zap.IncreaseLevel(level),
		)
		*(nl.Logger) = *newCore
	}

	for name, nl := range namedSugarLoggers {
		level := getLevel(name)
		newCore := zap.New(logger.Core()).Named(name).WithOptions(
			zap.IncreaseLevel(level),
		).Sugar()
		*(nl) = *newCore
	}
}

func Default() *zap.Logger {
	mu.Lock()
	defer mu.Unlock()
	return logger
}

// getLevel returns the level for the given name
// it return the first matching name or glob pattern whatever comes first
func getLevel(name string) zap.AtomicLevel {
	for _, nl := range namedLevels {
		if nl.name == name {
			return nl.level
		}
		if g, ok := namedGlobs[nl.name]; ok && g.Match(name) {
			return nl.level
		}
	}
	return zap.NewAtomicLevelAt(logger.Level())
}

func NewNamed(name string, fields ...zap.Field) CtxLogger {
	mu.Lock()
	defer mu.Unlock()

	if l, nameExists := namedLoggers[name]; nameExists {
		return l
	}

	level := getLevel(name)
	l := zap.New(logger.Core()).Named(name).WithOptions(zap.IncreaseLevel(level),
		zap.Fields(fields...))

	ctxL := CtxLogger{Logger: l, name: name}
	namedLoggers[name] = ctxL
	return ctxL
}

func NewNamedSugared(name string) *zap.SugaredLogger {
	mu.Lock()
	defer mu.Unlock()

	if l, nameExists := namedSugarLoggers[name]; nameExists {
		return l
	}

	level := getLevel(name)
	l := zap.New(logger.Core()).Named(name).Sugar().WithOptions(zap.IncreaseLevel(level))
	namedSugarLoggers[name] = l
	return l
}

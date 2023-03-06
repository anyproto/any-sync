package logger

import (
	"go.uber.org/zap"

	"github.com/anytypeio/any-sync/util/slice"
)

type Config struct {
	Production     bool              `yaml:"production"`
	DefaultLevel   string            `yaml:"defaultLevel"`
	NamedLevels    map[string]string `yaml:"namedLevels"`
	AddOutputPaths []string
	DisableStdErr  bool
}

func (l Config) ApplyGlobal() {
	var conf zap.Config
	if l.Production {
		conf = zap.NewProductionConfig()
	} else {
		conf = zap.NewDevelopmentConfig()
	}
	if len(l.AddOutputPaths) > 0 {
		conf.OutputPaths = append(conf.OutputPaths, l.AddOutputPaths...)
	}
	if l.DisableStdErr {
		conf.OutputPaths = slice.Filter(conf.OutputPaths, func(path string) bool {
			return path != "stderr"
		})
	}

	var err error
	if defaultLevel, err := zap.ParseAtomicLevel(l.DefaultLevel); err == nil {
		conf.Level = defaultLevel
	}
	var lvl = make(map[string]zap.AtomicLevel)
	for k, v := range l.NamedLevels {
		if lev, err := zap.ParseAtomicLevel(v); err == nil {
			lvl[k] = lev
			// we need to have a minimum level of all named loggers for the main logger
			if lev.Level() < conf.Level.Level() {
				conf.Level.SetLevel(lev.Level())
			}
		}
	}
	lg, err := conf.Build()
	if err != nil {
		Default().Fatal("can't build logger", zap.Error(err))
	}
	SetDefault(lg)
	SetNamedLevels(lvl)
}

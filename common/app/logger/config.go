package logger

import "go.uber.org/zap"

type Config struct {
	Production   bool              `yaml:"production"`
	DefaultLevel string            `yaml:"defaultLevel"`
	NamedLevels  map[string]string `yaml:"namedLevels"`
}

func (l Config) ApplyGlobal() {
	var conf zap.Config
	if l.Production {
		conf = zap.NewProductionConfig()
	} else {
		conf = zap.NewDevelopmentConfig()
	}
	if defaultLevel, err := zap.ParseAtomicLevel(l.DefaultLevel); err == nil {
		conf.Level = defaultLevel
	}
	var levels = make(map[string]zap.AtomicLevel)
	for k, v := range l.NamedLevels {
		if lev, err := zap.ParseAtomicLevel(v); err != nil {
			levels[k] = lev
		}
	}
	defaultLogger, err := conf.Build()
	if err != nil {
		Default().Fatal("can't build logger", zap.Error(err))
	}
	SetDefault(defaultLogger)
	SetNamedLevels(levels)
}

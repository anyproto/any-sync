package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/anytypeio/any-sync/util/slice"
)

type LogFormat int

const (
	ColorizedOutput LogFormat = iota
	PlaintextOutput
	JSONOutput
)

type Config struct {
	Production     bool              `yaml:"production"`
	DefaultLevel   string            `yaml:"defaultLevel"`
	NamedLevels    map[string]string `yaml:"namedLevels"`
	AddOutputPaths []string          `yaml:"outputPaths"`
	DisableStdErr  bool              `yaml:"disableStdErr"`
	Format         LogFormat         `yaml:"format"`
	ZapConfig      *zap.Config       `yaml:"-"` // optional, if set it will be used instead of other config options
}

func (l Config) ApplyGlobal() {
	var conf zap.Config
	if l.ZapConfig != nil {
		conf = *l.ZapConfig
	} else {
		if l.Production {
			conf = zap.NewProductionConfig()
		} else {
			conf = zap.NewDevelopmentConfig()
		}
		var encConfig zapcore.EncoderConfig
		switch l.Format {
		case PlaintextOutput:
			encConfig.EncodeLevel = zapcore.CapitalLevelEncoder
			conf.Encoding = "console"
		case JSONOutput:
			encConfig.MessageKey = "msg"
			encConfig.TimeKey = "ts"
			encConfig.LevelKey = "level"
			encConfig.NameKey = "logger"
			encConfig.CallerKey = "caller"
			encConfig.EncodeTime = zapcore.ISO8601TimeEncoder
			conf.Encoding = "json"
		default:
			// default is ColorizedOutput
			conf.Encoding = "console"
			encConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		}

		conf.EncoderConfig = encConfig
		if len(l.AddOutputPaths) > 0 {
			conf.OutputPaths = append(conf.OutputPaths, l.AddOutputPaths...)
		}
		if l.DisableStdErr {
			conf.OutputPaths = slice.Filter(conf.OutputPaths, func(path string) bool {
				return path != "stderr"
			})
		}

		if defaultLevel, err := zap.ParseAtomicLevel(l.DefaultLevel); err == nil {
			conf.Level = defaultLevel
		}
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

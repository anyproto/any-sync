package logger

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/anyproto/any-sync/util/slice"
)

type LogFormat int

const (
	ColorizedOutput LogFormat = iota
	PlaintextOutput
	JSONOutput
)

type NamedLevel struct {
	Name  string `yaml:"name"`
	Level string `yaml:"level"`
}

type Config struct {
	Production     bool         `yaml:"production"`
	DefaultLevel   string       `yaml:"defaultLevel"`
	Levels         []NamedLevel `yaml:"levels"` // first match will be used
	AddOutputPaths []string     `yaml:"outputPaths"`
	DisableStdErr  bool         `yaml:"disableStdErr"`
	Format         LogFormat    `yaml:"format"`
	ZapConfig      *zap.Config  `yaml:"-"` // optional, if set it will be used instead of other config options
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
		encConfig := conf.EncoderConfig
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
	for _, v := range l.Levels {
		if lev, err := zap.ParseAtomicLevel(v.Level); err == nil {
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
	SetNamedLevels(l.Levels)
}

// LevelsFromStr parses a string of the form "name1=DEBUG;prefix*=WARN;*=ERROR" into a slice of NamedLevel
// it may be useful to parse the log level from the OS env var
func LevelsFromStr(s string) (levels []NamedLevel) {
	for _, kv := range strings.Split(s, ";") {
		strings.TrimSpace(kv)
		parts := strings.Split(kv, "=")
		var key, value string
		if len(parts) == 1 {
			key = "*"
			value = parts[0]
			_, err := zap.ParseAtomicLevel(value)
			if err != nil {
				fmt.Printf("Can't parse log level %s: %s\n", parts[0], err.Error())
				continue
			}
			levels = append(levels, NamedLevel{Name: key, Level: value})
		} else if len(parts) == 2 {
			key = parts[0]
			value = parts[1]
		}
		_, err := zap.ParseAtomicLevel(value)
		if err != nil {
			fmt.Printf("Can't parse log level %s: %s\n", parts[0], err.Error())
			continue
		}
		levels = append(levels, NamedLevel{Name: key, Level: value})
	}
	return levels
}

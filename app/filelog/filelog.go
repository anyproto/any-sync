package filelog

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/debugstat"
)

const CName = "common.filelog"

type LogFile struct {
	Log string `json:"log"`
}

type FileLogger interface {
	app.ComponentRunnable
	debugstat.StatProvider
	DoLog(fn func(logger *zap.Logger))
}

type fileLogger struct {
	filePath    string
	statService debugstat.StatService
	logger      *zap.Logger
	file        *os.File
	mu          sync.RWMutex
}

func New(filePath string) FileLogger {
	return &fileLogger{
		filePath: filePath,
	}
}

func NewNoOp() FileLogger {
	return &noOpFileLogger{}
}

func (fl *fileLogger) Init(a *app.App) error {
	if fl.filePath == "" {
		return fmt.Errorf("filePath cannot be empty")
	}

	file, err := os.OpenFile(fl.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	fl.file = file

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(file),
		zapcore.DebugLevel,
	)

	fl.logger = zap.New(core, zap.AddCaller())

	if statService, ok := a.Component(debugstat.CName).(debugstat.StatService); ok {
		fl.statService = statService
	} else {
		fl.statService = debugstat.NewNoOp()
	}
	fl.statService.AddProvider(fl)

	return nil
}

func (fl *fileLogger) Name() string {
	return CName
}

func (fl *fileLogger) Run(ctx context.Context) error {
	return nil
}

func (fl *fileLogger) Close(ctx context.Context) error {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	if fl.logger != nil {
		_ = fl.logger.Sync()
	}
	if fl.file != nil {
		return fl.file.Close()
	}
	if fl.statService != nil {
		fl.statService.RemoveProvider(fl)
	}
	return nil
}

func (fl *fileLogger) DoLog(fn func(logger *zap.Logger)) {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	if fl.logger != nil {
		fn(fl.logger)
	}
}

func (fl *fileLogger) ProvideStat() any {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	content, err := os.ReadFile(fl.filePath)
	if err != nil {
		return LogFile{
			Log: fmt.Sprintf("error: failed to read log file: %v", err),
		}
	}

	encoded := base64.StdEncoding.EncodeToString(content)

	return LogFile{
		Log: encoded,
	}
}

func (fl *fileLogger) StatId() string {
	return fl.filePath
}

func (fl *fileLogger) StatType() string {
	return "filelog"
}

type noOpFileLogger struct{}

func (n *noOpFileLogger) Init(a *app.App) error {
	return nil
}

func (n *noOpFileLogger) Name() string {
	return CName
}

func (n *noOpFileLogger) Run(ctx context.Context) error {
	return nil
}

func (n *noOpFileLogger) Close(ctx context.Context) error {
	return nil
}

func (n *noOpFileLogger) DoLog(fn func(logger *zap.Logger)) {}

func (n *noOpFileLogger) ProvideStat() any {
	return LogFile{
		Log: "",
	}
}

func (n *noOpFileLogger) StatId() string {
	return "noop"
}

func (n *noOpFileLogger) StatType() string {
	return "filelog"
}

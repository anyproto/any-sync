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
	Log  string `json:"log"`
	Qlog string `json:"qlog"`
}

type FileLogger interface {
	app.ComponentRunnable
	debugstat.StatProvider
	DoLog(fn func(logger *zap.Logger))
	GetQlogFile() *os.File
}

type fileLogger struct {
	folderPath  string
	statService debugstat.StatService
	logger      *zap.Logger
	logFile     *os.File
	qlogFile    *os.File
	mu          sync.RWMutex
}

func New(folderPath string) FileLogger {
	return &fileLogger{
		folderPath: folderPath,
	}
}

func NewNoOp() FileLogger {
	return &noOpFileLogger{}
}

func (fl *fileLogger) Init(a *app.App) error {
	if fl.folderPath == "" {
		return fmt.Errorf("folderPath cannot be empty")
	}

	// Create folder if it doesn't exist
	if err := os.MkdirAll(fl.folderPath, 0755); err != nil {
		return fmt.Errorf("failed to create log folder: %w", err)
	}

	// Open log file
	logFilePath := fl.folderPath + "/app.log"
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	fl.logFile = logFile

	// Open qlog file
	qlogFilePath := fl.folderPath + "/quic.qlog"
	qlogFile, err := os.OpenFile(qlogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open qlog file: %w", err)
	}
	fl.qlogFile = qlogFile

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
		zapcore.AddSync(logFile),
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
	var err error
	if fl.logFile != nil {
		err = fl.logFile.Close()
	}
	if fl.qlogFile != nil {
		if qErr := fl.qlogFile.Close(); qErr != nil && err == nil {
			err = qErr
		}
	}
	if err != nil {
		return err
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

func (fl *fileLogger) GetQlogFile() *os.File {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	return fl.qlogFile
}

func (fl *fileLogger) ProvideStat() any {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	// Read log file
	logFilePath := fl.folderPath + "/app.log"
	logContent, err := os.ReadFile(logFilePath)
	if err != nil {
		logContent = []byte(fmt.Sprintf("error: failed to read log file: %v", err))
	}
	encodedLog := base64.StdEncoding.EncodeToString(logContent)

	// Read qlog file
	qlogFilePath := fl.folderPath + "/quic.qlog"
	qlogContent, err := os.ReadFile(qlogFilePath)
	if err != nil {
		qlogContent = []byte(fmt.Sprintf("error: failed to read qlog file: %v", err))
	}
	encodedQlog := base64.StdEncoding.EncodeToString(qlogContent)

	return LogFile{
		Log:  encodedLog,
		Qlog: encodedQlog,
	}
}

func (fl *fileLogger) StatId() string {
	return fl.folderPath
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

func (n *noOpFileLogger) GetQlogFile() *os.File {
	return nil
}

func (n *noOpFileLogger) ProvideStat() any {
	return LogFile{
		Log:  "",
		Qlog: "",
	}
}

func (n *noOpFileLogger) StatId() string {
	return "noop"
}

func (n *noOpFileLogger) StatType() string {
	return "filelog"
}

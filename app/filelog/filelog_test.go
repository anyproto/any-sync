package filelog

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app"
)

func TestFileLogger(t *testing.T) {
	t.Run("New and Init", func(t *testing.T) {
		tmpDir := t.TempDir()
		
		fl := New(tmpDir)
		require.NotNil(t, fl)
		
		a := &app.App{}
		err := fl.Init(a)
		require.NoError(t, err)
		
		assert.Equal(t, CName, fl.Name())
		
		err = fl.Close(context.Background())
		require.NoError(t, err)
	})
	
	t.Run("Init with empty path", func(t *testing.T) {
		fl := New("")
		a := &app.App{}
		err := fl.Init(a)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "folderPath cannot be empty")
	})
	
	t.Run("DoLog", func(t *testing.T) {
		tmpDir := t.TempDir()
		
		fl := New(tmpDir)
		a := &app.App{}
		err := fl.Init(a)
		require.NoError(t, err)
		
		fl.DoLog(func(logger *zap.Logger) {
			logger.Info("test message", zap.String("key", "value"))
		})
		
		err = fl.Close(context.Background())
		require.NoError(t, err)
		
		logPath := filepath.Join(tmpDir, "app.log")
		content, err := os.ReadFile(logPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "test message")
		assert.Contains(t, string(content), "key")
		assert.Contains(t, string(content), "value")
	})
	
	t.Run("ProvideStat", func(t *testing.T) {
		tmpDir := t.TempDir()
		
		fl := New(tmpDir)
		a := &app.App{}
		err := fl.Init(a)
		require.NoError(t, err)
		
		testContent := "test log content"
		fl.DoLog(func(logger *zap.Logger) {
			logger.Info(testContent)
		})
		
		err = fl.Close(context.Background())
		require.NoError(t, err)
		
		fl = New(tmpDir)
		err = fl.Init(a)
		require.NoError(t, err)
		
		stat := fl.ProvideStat()
		logFile, ok := stat.(LogFile)
		require.True(t, ok)
		
		decoded, err := base64.StdEncoding.DecodeString(logFile.Log)
		require.NoError(t, err)
		assert.Contains(t, string(decoded), testContent)
		
		assert.Equal(t, tmpDir, fl.StatId())
		assert.Equal(t, "filelog", fl.StatType())
		
		err = fl.Close(context.Background())
		require.NoError(t, err)
	})
	
	t.Run("Run", func(t *testing.T) {
		tmpDir := t.TempDir()
		
		fl := New(tmpDir)
		a := &app.App{}
		err := fl.Init(a)
		require.NoError(t, err)
		
		err = fl.Run(context.Background())
		require.NoError(t, err)
		
		err = fl.Close(context.Background())
		require.NoError(t, err)
	})
}

func TestNoOpFileLogger(t *testing.T) {
	t.Run("Basic operations", func(t *testing.T) {
		fl := NewNoOp()
		require.NotNil(t, fl)
		
		a := &app.App{}
		err := fl.Init(a)
		require.NoError(t, err)
		
		assert.Equal(t, CName, fl.Name())
		
		err = fl.Run(context.Background())
		require.NoError(t, err)
		
		fl.DoLog(func(logger *zap.Logger) {
			t.Fatal("DoLog should not call the function in NoOp implementation")
		})
		
		err = fl.Close(context.Background())
		require.NoError(t, err)
	})
	
	t.Run("ProvideStat", func(t *testing.T) {
		fl := NewNoOp()
		
		stat := fl.ProvideStat()
		logFile, ok := stat.(LogFile)
		require.True(t, ok)
		assert.Equal(t, "", logFile.Log)
		assert.Equal(t, "", logFile.Qlog)
		
		assert.Equal(t, "noop", fl.StatId())
		assert.Equal(t, "filelog", fl.StatType())
	})
}
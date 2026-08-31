package peerobserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/anyproto/any-sync/app/logger"
)

// captureLogs rebinds the package's named logger onto an observed core, so
// the tests can assert on what Notify logs
func captureLogs(t *testing.T) *observer.ObservedLogs {
	orig := *logger.Default()
	core, logs := observer.New(zap.InfoLevel)
	logger.SetDefault(zap.New(core))
	logger.SetNamedLevels(nil)
	t.Cleanup(func() {
		logger.SetDefault(&orig)
		logger.SetNamedLevels(nil)
	})
	return logs
}

func TestNotify(t *testing.T) {
	t.Run("nil observer produces nothing", func(t *testing.T) {
		logs := captureLogs(t)

		Notify(nil, Event{Kind: KindConnected, PeerId: "p1"})

		// no recovered panic, no log line: the guard, not the recover, must
		// handle the absent observer
		assert.Zero(t, logs.Len())
	})
	t.Run("panicking observer is contained and logged", func(t *testing.T) {
		logs := captureLogs(t)

		Notify(panicky{}, Event{Kind: KindConnected, PeerId: "p1"})

		require.Equal(t, 1, logs.FilterMessage("peer observer panic").Len())
	})
	t.Run("typed-nil observer panics are contained per event", func(t *testing.T) {
		logs := captureLogs(t)
		var obs *fieldObserver // typed nil defeats the == nil guard

		Notify(obs, Event{Kind: KindConnected, PeerId: "p1"})

		require.Equal(t, 1, logs.FilterMessage("peer observer panic").Len())
	})
}

func TestNew(t *testing.T) {
	comp := New(panicky{})
	assert.Equal(t, CName, comp.Name())
	assert.NoError(t, comp.Init(nil))
	_, isObserver := comp.(Observer)
	assert.True(t, isObserver)

	t.Run("nil observer is inert", func(t *testing.T) {
		logs := captureLogs(t)
		obs := New(nil).(Observer)

		Notify(obs, Event{Kind: KindConnected, PeerId: "p1"})

		assert.Zero(t, logs.Len(), "a nil observer must produce no work and no logs per event")
	})
}

type panicky struct{}

func (panicky) ObservePeerEvent(Event) { panic("observer panic") }

type fieldObserver struct {
	calls int
}

func (f *fieldObserver) ObservePeerEvent(Event) { f.calls++ }

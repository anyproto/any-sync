package app

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type probeComp struct {
	name     string
	failInit bool
	failRun  bool
	inits    int
	closes   int
}

func (c *probeComp) Init(a *App) error {
	c.inits++
	if c.failInit {
		return errors.New("boom")
	}
	return nil
}
func (c *probeComp) Name() string { return c.name }
func (c *probeComp) Run(ctx context.Context) error {
	if c.failRun {
		return errors.New("boom")
	}
	return nil
}
func (c *probeComp) Close(ctx context.Context) error { c.closes++; return nil }

func (c *probeComp) assertCounts(t *testing.T, inits, closes int) {
	t.Helper()
	assert.Equal(t, inits, c.inits, "%s: Init count", c.name)
	assert.Equal(t, closes, c.closes, "%s: Close count", c.name)
}

// A caller whose Start failed still closes the app (commonspace does:
// space.Init is app.Start, and a failed Init is followed by space.Close).
func TestStartInitFailureSkipsUninitialized(t *testing.T) {
	a := new(App)
	before := &probeComp{name: "before"}
	failing := &probeComp{name: "failing", failInit: true}
	after := &probeComp{name: "after"}
	a.Register(before).Register(failing).Register(after)

	require.Error(t, a.Start(context.Background()))
	require.NoError(t, a.Close(context.Background()))

	before.assertCounts(t, 1, 1)
	failing.assertCounts(t, 1, 1)
	// Never ran Init, so its fields are nil: closing it panics.
	after.assertCounts(t, 0, 0)
}

// Run failure: EVERY component ran Init, so every one still holds Init-time
// resources and must be closed exactly once.
func TestStartRunFailureClosesEveryInitialized(t *testing.T) {
	a := new(App)
	before := &probeComp{name: "before"}
	failing := &probeComp{name: "failing", failRun: true}
	after := &probeComp{name: "after"}
	a.Register(before).Register(failing).Register(after)

	require.Error(t, a.Start(context.Background()))
	require.NoError(t, a.Close(context.Background()))

	before.assertCounts(t, 1, 1)
	failing.assertCounts(t, 1, 1)
	after.assertCounts(t, 1, 1)
}

// Concurrent Close storm: exactly one cleanup pass, no torn state.
// Mirrors a caller leaking a close goroutine while another close runs.
func TestConcurrentCloseRunsOnce(t *testing.T) {
	for round := 0; round < 200; round++ {
		a := new(App)
		before := &probeComp{name: "before"}
		failing := &probeComp{name: "failing", failInit: true}
		after := &probeComp{name: "after"}
		a.Register(before).Register(failing).Register(after)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() { defer wg.Done(); _ = a.Start(context.Background()) }()
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); _ = a.Close(context.Background()) }()
		}
		wg.Wait()

		if after.closes != 0 {
			t.Fatalf("round %d: never-Init'd component closed %d times", round, after.closes)
		}
		if before.closes > 1 || failing.closes > 1 {
			t.Fatalf("round %d: double close before=%d failing=%d", round, before.closes, failing.closes)
		}
	}
}

func TestDoubleStartRejected(t *testing.T) {
	a := new(App)
	c := &probeComp{name: "c"}
	a.Register(c)

	require.NoError(t, a.Start(context.Background()))
	require.ErrorIs(t, a.Start(context.Background()), ErrAppAlreadyStarted)
	require.NoError(t, a.Close(context.Background()))

	c.assertCounts(t, 1, 1)
}

// Concurrent Start storm: exactly one wins, components Init once.
func TestConcurrentStartRunsOnce(t *testing.T) {
	for round := 0; round < 200; round++ {
		a := new(App)
		c := &probeComp{name: "c"}
		a.Register(c)

		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); _ = a.Start(context.Background()) }()
		}
		wg.Wait()
		if c.inits != 1 {
			t.Fatalf("round %d: Init ran %d times, want 1", round, c.inits)
		}
	}
}

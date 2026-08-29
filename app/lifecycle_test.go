package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

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

		// The invariant, whichever goroutine won: a component whose Init
		// ran is closed exactly once, one that never ran Init is never
		// closed. (If a Close won the lock first, Start is rejected and
		// nothing is Init'd — also valid.)
		for _, c := range []*probeComp{before, failing, after} {
			want := 0
			if c.inits > 0 {
				want = 1
			}
			if c.closes != want {
				t.Fatalf("round %d: %s init=%d close=%d, want close=%d",
					round, c.name, c.inits, c.closes, want)
			}
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

// nilFieldComp models a real component (headSync): Close touches a field that
// only Init populates, so closing it mid-Init panics.
type nilFieldComp struct {
	name   string
	dep    *struct{ x int }
	closes int
	onInit func()
	mu     sync.Mutex
}

func (c *nilFieldComp) Init(a *App) error {
	if c.onInit != nil {
		c.onInit()
	}
	c.dep = &struct{ x int }{}
	return nil
}
func (c *nilFieldComp) Name() string                  { return c.name }
func (c *nilFieldComp) Run(ctx context.Context) error { return nil }
func (c *nilFieldComp) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closes++
	_ = c.dep.x // panics if Init never completed
	return nil
}

// Close must not run against a component whose Init is still in flight — it
// waits for Start instead. Without the lifecycle lock this panics with the
// same nil deref the Init high-water mark exists to prevent.
func TestCloseWaitsForInFlightStart(t *testing.T) {
	gate := make(chan struct{})
	entered := make(chan struct{})
	c := &nilFieldComp{name: "c", onInit: func() { close(entered); <-gate }}
	a := new(App)
	a.Register(c)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); assert.NoError(t, a.Start(context.Background())) }()
	<-entered // Start is parked inside c.Init

	closed := make(chan error, 1)
	go func() { closed <- a.Close(context.Background()) }()

	select {
	case <-closed:
		t.Fatal("Close returned while Init was still running")
	case <-time.After(50 * time.Millisecond):
	}
	close(gate)
	wg.Wait()
	require.NoError(t, <-closed)

	c.mu.Lock()
	defer c.mu.Unlock()
	assert.Equal(t, 1, c.closes, "component closed exactly once, after Init finished")
}

// A Close landing before Start must not silently consume the app's cleanup:
// Start refuses, so nothing is ever Init'd and left unreachable.
func TestStartAfterCloseIsRejected(t *testing.T) {
	a := new(App)
	c1 := &probeComp{name: "c1"}
	c2 := &probeComp{name: "c2"}
	a.Register(c1).Register(c2)

	require.NoError(t, a.Close(context.Background()))
	require.ErrorIs(t, a.Start(context.Background()), ErrAppClosed)

	c1.assertCounts(t, 0, 0)
	c2.assertCounts(t, 0, 0)
}

// Close on an app that was never started closes nothing — every component's
// fields are still nil. This is the case the initialized bound protects.
func TestCloseWithoutStartClosesNothing(t *testing.T) {
	a := new(App)
	c := &nilFieldComp{name: "c"}
	a.Register(c)

	require.NoError(t, a.Close(context.Background()))
	assert.Equal(t, 0, c.closes)
}

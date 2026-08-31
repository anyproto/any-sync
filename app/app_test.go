package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppServiceRegistry(t *testing.T) {
	app := new(App)
	t.Run("Register", func(t *testing.T) {
		app.Register(newTestService(testTypeRunnable, "c1", nil, nil))
		app.Register(newTestService(testTypeRunnable, "r1", nil, nil))
		app.Register(newTestService(testTypeComponent, "s1", nil, nil))
	})
	t.Run("Component", func(t *testing.T) {
		assert.Nil(t, app.Component("not-registered"))
		for _, name := range []string{"c1", "r1", "s1"} {
			s := app.Component(name)
			assert.NotNil(t, s, name)
			assert.Equal(t, name, s.Name())
		}
	})
	t.Run("MustComponent", func(t *testing.T) {
		for _, name := range []string{"c1", "r1", "s1"} {
			assert.NotPanics(t, func() { app.MustComponent(name) }, name)
		}
		assert.Panics(t, func() { app.MustComponent("not-registered") })
	})
	t.Run("ComponentNames", func(t *testing.T) {
		names := app.ComponentNames()
		assert.Equal(t, names, []string{"c1", "r1", "s1"})
	})
	t.Run("Child MustComponent", func(t *testing.T) {
		app := app.ChildApp()
		app.Register(newTestService(testTypeComponent, "x1", nil, nil))
		for _, name := range []string{"c1", "r1", "s1", "x1"} {
			assert.NotPanics(t, func() { app.MustComponent(name) }, name)
		}
		assert.Panics(t, func() { app.MustComponent("not-registered") })
	})
	t.Run("Child ComponentNames", func(t *testing.T) {
		app := app.ChildApp()
		app.Register(newTestService(testTypeComponent, "x1", nil, nil))
		names := app.ComponentNames()
		assert.Equal(t, names, []string{"x1", "c1", "r1", "s1"})
	})
	t.Run("Child override", func(t *testing.T) {
		app := app.ChildApp()
		app.Register(newTestService(testTypeRunnable, "s1", nil, nil))
		_ = app.MustComponent("s1").(*testRunnable)
	})
}

func TestApp_IterateComponents(t *testing.T) {
	app := new(App)

	app.Register(newTestService(testTypeRunnable, "c1", nil, nil))
	app.Register(newTestService(testTypeRunnable, "r1", nil, nil))
	app.Register(newTestService(testTypeComponent, "s1", nil, nil))

	var got []string
	app.IterateComponents(func(s Component) {
		got = append(got, s.Name())
	})

	assert.ElementsMatch(t, []string{"c1", "r1", "s1"}, got)
}

func TestAppStart(t *testing.T) {
	t.Run("SuccessStartStop", func(t *testing.T) {
		app := new(App)
		seq := new(testSeq)
		services := [...]iTestService{
			newTestService(testTypeRunnable, "c1", nil, seq),
			newTestService(testTypeRunnable, "r1", nil, seq),
			newTestService(testTypeComponent, "s1", nil, seq),
			newTestService(testTypeRunnable, "c2", nil, seq),
		}
		for _, s := range services {
			app.Register(s)
		}
		ctx := context.Background()
		assert.Nil(t, app.Start(ctx))
		assert.Nil(t, app.Close(ctx))

		var actual []testIds
		for _, s := range services {
			actual = append(actual, s.Ids())
		}

		expected := []testIds{
			{1, 5, 10},
			{2, 6, 9},
			{3, 0, 0},
			{4, 7, 8},
		}

		assert.Equal(t, expected, actual)
	})

	t.Run("InitError", func(t *testing.T) {
		app := new(App)
		seq := new(testSeq)
		expectedErr := fmt.Errorf("testError")
		services := [...]iTestService{
			newTestService(testTypeRunnable, "c1", nil, seq),
			newTestService(testTypeRunnable, "c2", expectedErr, seq),
		}
		for _, s := range services {
			app.Register(s)
		}

		err := app.Start(context.Background())
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), expectedErr.Error())

		var actual []testIds
		for _, s := range services {
			actual = append(actual, s.Ids())
		}

		expected := []testIds{
			{1, 0, 4},
			{2, 0, 3},
		}
		assert.Equal(t, expected, actual)
	})
}

const (
	testTypeComponent int = iota
	testTypeRunnable
)

func newTestService(componentType int, name string, err error, seq *testSeq) (s iTestService) {
	switch componentType {
	case testTypeComponent:
		return &testComponent{name: name, err: err, seq: seq}
	case testTypeRunnable:
		return &testRunnable{testComponent: testComponent{name: name, err: err, seq: seq}}
	}
	return nil
}

type iTestService interface {
	Component
	Ids() (ids testIds)
}

type testIds struct {
	initId  int64
	runId   int64
	closeId int64
}

type testComponent struct {
	name string
	err  error
	// runErr fails Run only, so a Run-phase failure can be exercised
	// independently of Init.
	runErr error
	seq    *testSeq
	ids    testIds
	// onInit runs before dep is populated, so a test can park a component
	// mid-Init.
	onInit func()
	// dep stands in for the fields a real component populates in Init and
	// dereferences in Close (headSync.syncer, treeBuilder's deps): closing a
	// component whose Init never ran panics, exactly as in production.
	dep    *struct{ x int }
	mu     sync.Mutex
	inits  int
	closes int
}

func (t *testComponent) Init(a *App) error {
	t.mu.Lock()
	t.inits++
	t.mu.Unlock()
	t.ids.initId = t.seq.New()
	if t.onInit != nil {
		t.onInit()
	}
	// Set even when Init reports an error: a component that fails Init is
	// still closed, so it must survive that close.
	t.dep = &struct{ x int }{}
	return t.err
}

// counts reports how many times Init and Close ran.
func (t *testComponent) counts() (inits, closes int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.inits, t.closes
}

func (t *testComponent) assertCounts(tb testing.TB, inits, closes int) {
	tb.Helper()
	gotInits, gotCloses := t.counts()
	assert.Equal(tb, inits, gotInits, "%s: Init count", t.name)
	assert.Equal(tb, closes, gotCloses, "%s: Close count", t.name)
}

func (t *testComponent) Name() string { return t.name }

func (t *testComponent) Ids() testIds {
	return t.ids
}

type testRunnable struct {
	testComponent
}

func (t *testRunnable) Run(ctx context.Context) error {
	t.ids.runId = t.seq.New()
	return t.runErr
}

func (t *testRunnable) Close(ctx context.Context) error {
	t.mu.Lock()
	t.closes++
	t.mu.Unlock()
	t.ids.closeId = t.seq.New()
	_ = t.dep.x // panics if Init never populated this component
	return t.err
}

type testSeq struct {
	seq int64
}

func (ts *testSeq) New() int64 {
	return atomic.AddInt64(&ts.seq, 1)
}

func newRunnable(name string, seq *testSeq) *testRunnable {
	return &testRunnable{testComponent: testComponent{name: name, seq: seq}}
}

// A caller whose Start failed still closes the app (commonspace does: space.Init
// is app.Start, and a failed Init is followed by space.Close). Components past
// the failure never ran Init, so closing them would panic.
func TestStartInitFailureSkipsUninitialized(t *testing.T) {
	seq := new(testSeq)
	before, failing, after := newRunnable("before", seq), newRunnable("failing", seq), newRunnable("after", seq)
	failing.err = errors.New("init boom")

	a := new(App)
	a.Register(before).Register(failing).Register(after)

	require.Error(t, a.Start(context.Background()))
	require.NoError(t, a.Close(context.Background()))

	before.assertCounts(t, 1, 1)
	failing.assertCounts(t, 1, 1)
	after.assertCounts(t, 0, 0)
}

// A Run failure is different: every component ran Init, so every one holds
// Init-time resources and must be closed exactly once.
func TestStartRunFailureClosesEveryInitialized(t *testing.T) {
	seq := new(testSeq)
	before, failing, after := newRunnable("before", seq), newRunnable("failing", seq), newRunnable("after", seq)
	failing.runErr = errors.New("run boom")

	a := new(App)
	a.Register(before).Register(failing).Register(after)

	require.Error(t, a.Start(context.Background()))
	require.NoError(t, a.Close(context.Background()))

	before.assertCounts(t, 1, 1)
	failing.assertCounts(t, 1, 1)
	after.assertCounts(t, 1, 1)
}

// Close must not run against a component whose Init is still in flight — it
// waits for Start instead. Without the lifecycle lock this panics with the nil
// deref the Init high-water mark exists to prevent.
func TestCloseWaitsForInFlightStart(t *testing.T) {
	gate, entered := make(chan struct{}), make(chan struct{})
	c := newRunnable("c", new(testSeq))
	c.onInit = func() { close(entered); <-gate }

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

	c.assertCounts(t, 1, 1)
}

// A Close landing before Start must not silently consume the app's one cleanup
// pass, leaving everything Start goes on to Init unreachable.
func TestStartAfterCloseIsRejected(t *testing.T) {
	seq := new(testSeq)
	c1, c2 := newRunnable("c1", seq), newRunnable("c2", seq)

	a := new(App)
	a.Register(c1).Register(c2)

	require.NoError(t, a.Close(context.Background()))
	require.ErrorIs(t, a.Start(context.Background()), ErrAppClosed)

	c1.assertCounts(t, 0, 0)
	c2.assertCounts(t, 0, 0)
}

// Close on an app that was never started closes nothing — every component's
// fields are still nil. This is the case the initialized bound protects.
func TestCloseWithoutStartClosesNothing(t *testing.T) {
	c := newRunnable("c", new(testSeq))

	a := new(App)
	a.Register(c)

	require.NoError(t, a.Close(context.Background()))
	c.assertCounts(t, 0, 0)
}

func TestDoubleStartRejected(t *testing.T) {
	c := newRunnable("c", new(testSeq))

	a := new(App)
	a.Register(c)

	require.NoError(t, a.Start(context.Background()))
	require.ErrorIs(t, a.Start(context.Background()), ErrAppAlreadyStarted)
	require.NoError(t, a.Close(context.Background()))

	c.assertCounts(t, 1, 1)
}

// Concurrent Close storm, mirroring a caller leaking a close goroutine while
// another close runs.
func TestConcurrentCloseRunsOnce(t *testing.T) {
	for round := 0; round < 200; round++ {
		seq := new(testSeq)
		before, failing, after := newRunnable("before", seq), newRunnable("failing", seq), newRunnable("after", seq)
		failing.err = errors.New("init boom")

		a := new(App)
		a.Register(before).Register(failing).Register(after)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() { defer wg.Done(); _ = a.Start(context.Background()) }()
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); _ = a.Close(context.Background()) }()
		}
		wg.Wait()

		// The invariant, whichever goroutine won: a component whose Init ran
		// is closed exactly once, one that never ran Init is never closed. (A
		// Close winning the lock first rejects Start, so nothing is Init'd —
		// also valid.)
		for _, c := range []*testRunnable{before, failing, after} {
			inits, closes := c.counts()
			want := 0
			if inits > 0 {
				want = 1
			}
			if closes != want {
				t.Fatalf("round %d: %s init=%d close=%d, want close=%d", round, c.name, inits, closes, want)
			}
		}
	}
}

// Concurrent Start storm: exactly one wins, components Init once.
func TestConcurrentStartRunsOnce(t *testing.T) {
	for round := 0; round < 200; round++ {
		c := newRunnable("c", new(testSeq))
		a := new(App)
		a.Register(c)

		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); _ = a.Start(context.Background()) }()
		}
		wg.Wait()

		if inits, _ := c.counts(); inits != 1 {
			t.Fatalf("round %d: Init ran %d times, want 1", round, inits)
		}
	}
}

package app

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
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
	ts := testComponent{name: name, err: err, seq: seq}
	switch componentType {
	case testTypeComponent:
		return &ts
	case testTypeRunnable:
		return &testRunnable{testComponent: ts}
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
	seq  *testSeq
	ids  testIds
}

func (t *testComponent) Init(a *App) error {
	t.ids.initId = t.seq.New()
	return t.err
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
	return t.err
}

func (t *testRunnable) Close(ctx context.Context) error {
	t.ids.closeId = t.seq.New()
	return t.err
}

type testSeq struct {
	seq int64
}

func (ts *testSeq) New() int64 {
	return atomic.AddInt64(&ts.seq, 1)
}

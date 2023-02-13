package secureservice

import (
	"context"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/testutil/accounttest"
	"github.com/stretchr/testify/require"
	"testing"
)

var ctx = context.Background()

func TestHandshake(t *testing.T) {
	fx := newFixture(t)
	defer fx.Finish(t)
}

func newFixture(t *testing.T) *fixture {
	fx := &fixture{
		secureService: New().(*secureService),
		a:             new(app.App),
	}
	fx.a.Register(&accounttest.AccountTestService{}).Register(fx.secureService)
	require.NoError(t, fx.a.Start(ctx))
	return fx
}

type fixture struct {
	*secureService
	a *app.App
}

func (fx *fixture) Finish(t *testing.T) {
	require.NoError(t, fx.a.Close(ctx))
}

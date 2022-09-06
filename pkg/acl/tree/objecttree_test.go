package tree

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"testing"
)

func TestObjectTree(t *testing.T) {
	a := &app.App{}
	inmemory := storage.NewInMemoryTreeStorage(...)
	app.RegisterWithType[storage.TreeStorage](a, inmemory)
	app.RegisterWithType[]()

	a.Start(context.Background())
	objectTree := app.MustComponentWithType[ObjectTree](a).(ObjectTree)



}

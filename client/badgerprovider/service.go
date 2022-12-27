package badgerprovider

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/dgraph-io/badger/v3"
)

type BadgerProvider interface {
	app.ComponentRunnable
	Badger() *badger.DB
}

var CName = "client.badgerprovider"

type service struct {
	db *badger.DB
}

func New() BadgerProvider {
	return &service{}
}

func (s *service) Init(a *app.App) (err error) {
	cfg := a.MustComponent("config").(configGetter).GetStorage()
	s.db, err = badger.Open(badger.DefaultOptions(cfg.Path))
	return
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Badger() *badger.DB {
	return s.db
}

func (s *service) Run(ctx context.Context) (err error) {
	return
}

func (s *service) Close(ctx context.Context) (err error) {
	return s.db.Close()
}

package debugstat

import (
	"context"

	"github.com/anyproto/any-sync/app"
)

func NewNoOp() StatService {
	return NoOpStatService{}
}

type NoOpStatService struct {
}

func (n NoOpStatService) Init(a *app.App) (err error) {
	return nil
}

func (n NoOpStatService) Name() (name string) {
	return CName
}

func (n NoOpStatService) Run(ctx context.Context) (err error) {
	return nil
}

func (n NoOpStatService) Close(ctx context.Context) (err error) {
	return nil
}

func (n NoOpStatService) AddProvider(provider StatProvider) {
	return
}

func (n NoOpStatService) RemoveProvider(provider StatProvider) {
	return
}

func (n NoOpStatService) GetStat() StatSummary {
	return StatSummary{}
}

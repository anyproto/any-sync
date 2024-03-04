//go:generate mockgen -destination mock_debugstat/mock_debugstat.go github.com/anyproto/any-sync/app/debugstat StatService
package debugstat

import (
	"context"
	"sync"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
)

var log = logger.NewNamed(CName)

const CName = "common.debugstat"

type StatProvider interface {
	ProvideStat() any
	StatId() string
	StatType() string
}

type AggregatableStatProvider interface {
	AggregateStat(stats []StatValue) any
}

type StatService interface {
	app.ComponentRunnable
	AddProvider(provider StatProvider)
	RemoveProvider(provider StatProvider)
	GetStat() StatSummary
}

func New() StatService {
	return &statService{}
}

type statService struct {
	providers map[string]StatProvider
	sync.Mutex
}

func (s *statService) AddProvider(provider StatProvider) {
	s.Lock()
	defer s.Unlock()
	s.providers[s.provId(provider)] = provider
}

func (s *statService) provId(provider StatProvider) string {
	return provider.StatType() + "-" + provider.StatId()
}

func (s *statService) RemoveProvider(provider StatProvider) {
	s.Lock()
	defer s.Unlock()
	delete(s.providers, s.provId(provider))
}

func (s *statService) Init(a *app.App) (err error) {
	s.providers = map[string]StatProvider{}
	return nil
}

func (s *statService) Name() (name string) {
	return CName
}

func (s *statService) GetStat() (st StatSummary) {
	s.Lock()
	allProviders := map[string][]StatProvider{}
	for _, prov := range s.providers {
		tp := prov.StatType()
		provs := allProviders[tp]
		provs = append(provs, prov)
		allProviders[tp] = provs
	}
	s.Unlock()

	for tp, provs := range allProviders {
		stType := StatType{
			Type: tp,
		}
		for _, prov := range provs {
			stat := prov.ProvideStat()
			stType.Values = append(stType.Values, StatValue{
				Key:   prov.StatId(),
				Value: stat,
			})
		}
		if len(provs) > 0 {
			if aggregate, ok := provs[0].(AggregatableStatProvider); ok {
				stType.Aggregate = aggregate.AggregateStat(stType.Values)
				stType.Values = nil
			}
		}
		st.Stats = append(st.Stats, stType)
	}
	return st
}

func (s *statService) Run(ctx context.Context) (err error) {
	// TODO: think if we should periodically collect stats and save them or log them
	return nil
}

func (s *statService) Close(ctx context.Context) (err error) {
	return nil
}

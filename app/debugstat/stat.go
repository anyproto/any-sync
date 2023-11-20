package debugstat

import (
	"context"
	"sync"
	"time"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/util/periodicsync"
)

var log = logger.NewNamed(CName)

const CName = "common.debugstat"

const (
	statCollectionSecs = 200
	statTimeout        = time.Second * 10
)

type StatProvider interface {
	ProvideStat() any
	StatId() string
	StatType() string
}

type StatService interface {
	app.ComponentRunnable
	AddProvider(provider StatProvider)
	RemoveProvider(provider StatProvider)
}

type statService struct {
	providers map[string]StatProvider
	updater   periodicsync.PeriodicSync
	curStat   statSummary
	sync.Mutex
}

func (s *statService) AddProvider(provider StatProvider) {
	s.Lock()
	defer s.Unlock()
	s.providers[provider.StatId()] = provider
}

func (s *statService) RemoveProvider(provider StatProvider) {
	s.Lock()
	defer s.Unlock()
	delete(s.providers, provider.StatId())
}

func (s *statService) Init(a *app.App) (err error) {
	s.providers = map[string]StatProvider{}
	s.updater = periodicsync.NewPeriodicSync(statCollectionSecs, statTimeout, s.loop, log)
	return nil
}

func (s *statService) Name() (name string) {
	return CName
}

func (s *statService) loop(ctx context.Context) (err error) {
	s.Lock()
	allProviders := map[string][]StatProvider{}
	for _, prov := range s.providers {
		tp := prov.StatType()
		provs := allProviders[tp]
		provs = append(provs, prov)
		allProviders[tp] = provs
	}
	s.Unlock()

	st := statSummary{}
	for tp, provs := range allProviders {
		stType := statType{
			Type:   tp,
			Values: nil,
		}
		for _, prov := range provs {
			stat := prov.ProvideStat()
			stType.Values = append(stType.Values, statValue{
				Key:   prov.StatId(),
				Value: stat,
			})
		}
		st.Stats = append(st.Stats, stType)
	}
	s.Lock()
	defer s.Unlock()
	s.curStat = st
	return nil
}

func (s *statService) Run(ctx context.Context) (err error) {
	s.updater.Run()
	return nil
}

func (s *statService) Close(ctx context.Context) (err error) {
	s.updater.Close()
	return nil
}

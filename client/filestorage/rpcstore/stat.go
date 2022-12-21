package rpcstore

import (
	"github.com/VividCortex/ewma"
	"sync"
	"time"
)

// defaultSpeedScore - initial value in Kb/s, it should be relatively high to test fresh client soon
const defaultSpeedScore = 10 * 1024

func newStat() *stat {
	s := &stat{
		ewma:      &ewma.SimpleEWMA{},
		lastUsage: time.Now(),
	}
	s.ewma.Set(float64(defaultSpeedScore))
	return s
}

// stat calculates EWMA download/upload speed
type stat struct {
	// TODO: rewrite to atomics
	ewma      *ewma.SimpleEWMA
	mu        sync.Mutex
	lastUsage time.Time
}

// Score returns average download/upload speed in Kb/s
func (s *stat) Score() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ewma.Value()
}

// Add adds new sample to the stat
func (s *stat) Add(startTime time.Time, byteLen int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dur := time.Since(startTime).Seconds()
	s.ewma.Add(float64(byteLen) / 1024 / dur)
	s.lastUsage = time.Now()
}

func (s *stat) LastUsage() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastUsage
}

func (s *stat) UpdateLastUsage() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastUsage = time.Now()
}

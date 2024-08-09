package globalsync

import (
	"fmt"
	"sync"

	"golang.org/x/exp/slices"
)

type Limit struct {
	peerStep      []int
	totalStep     []int
	excludeIds    []string
	excludedLimit int
	excludedTotal int
	counter       int
	total         int
	tokens        map[string]int
	mx            sync.Mutex
}

func NewLimit(peerStep, totalStep []int, excludeIds []string, excludedLimit int) *Limit {
	if len(peerStep) == 0 || len(totalStep) == 0 || len(peerStep) != len(totalStep)+1 {
		panic("incorrect limit configuration")
	}
	slices.SortFunc(peerStep, func(a, b int) int {
		if a < b {
			return 1
		} else if a > b {
			return -1
		} else {
			return 0
		}
	})
	slices.Sort(totalStep)
	// so here we would have something like
	// peerStep = [3, 2, 1]
	// totalStep = [3, 6], where everything more than 6 in total will get 1 token for each id
	totalStep = append(totalStep, totalStep[len(totalStep)-1])
	return &Limit{
		excludeIds:    excludeIds,
		excludedLimit: excludedLimit,
		peerStep:      peerStep,
		totalStep:     totalStep,
		tokens:        make(map[string]int),
	}
}

func (l *Limit) Take(id string) bool {
	l.mx.Lock()
	defer l.mx.Unlock()
	if l.isExcluded(id) {
		if l.tokens[id] >= l.excludedLimit {
			return false
		}
		l.tokens[id]++
		l.excludedTotal++
		return true
	}
	if l.tokens[id] >= l.peerStep[l.counter] {
		return false
	}
	l.tokens[id]++
	l.total++
	if l.total >= l.totalStep[l.counter] && l.counter < len(l.totalStep)-1 {
		l.counter++
	}
	return true
}

func (l *Limit) Release(id string) {
	l.mx.Lock()
	defer l.mx.Unlock()
	if l.tokens[id] > 0 {
		l.tokens[id]--
	} else {
		return
	}
	if l.isExcluded(id) {
		l.excludedTotal--
		return
	}
	l.total--
	if l.total < l.totalStep[l.counter] {
		if l.counter == len(l.totalStep)-1 {
			l.counter--
		}
		if l.counter > 0 {
			l.counter--
		}
	}
}

func (l *Limit) isExcluded(id string) bool {
	for _, excludeId := range l.excludeIds {
		if id == excludeId {
			return true
		}
	}
	return false
}

func (l *Limit) Stats(id string) string {
	l.mx.Lock()
	defer l.mx.Unlock()
	if l.isExcluded(id) {
		return fmt.Sprintf("excluded peer: %d/%d, total: %d/%d/%d", l.tokens[id], l.excludedLimit, l.excludedTotal, l.total, l.totalStep[l.counter])
	}
	return fmt.Sprintf("peer: %d/%d, total: %d/%d/%d", l.tokens[id], l.peerStep[l.counter], l.excludedTotal, l.total, l.totalStep[l.counter])
}

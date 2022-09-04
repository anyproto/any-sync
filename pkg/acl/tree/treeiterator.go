package tree

import (
	"sync"
)

var itPool = &sync.Pool{
	New: func() interface{} {
		return &iterator{
			stack:  make([]*Change, 0, 100),
			resBuf: make([]*Change, 0, 100),
		}
	},
}

func newIterator() *iterator {
	return itPool.Get().(*iterator)
}

func freeIterator(i *iterator) {
	itPool.Put(i)
}

type iterator struct {
	resBuf []*Change
	stack  []*Change
	f      func(c *Change) bool
}

func (i *iterator) iterateSkip(start *Change, skipBefore *Change, f func(c *Change) (isContinue bool)) {
	skipping := true
	i.iterate(start, func(c *Change) (isContinue bool) {
		if skipping && c != skipBefore {
			return true
		}
		skipping = false
		return f(c)
	})
}

func (i *iterator) topSort(start *Change) {
	stack := i.stack
	stack = append(stack, start)

	for len(stack) > 0 {
		ch := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if ch.visited {
			i.resBuf = append(i.resBuf, ch)
			continue
		}

		ch.visited = true
		stack = append(stack, ch)

		for j := 0; j < len(ch.Next); j++ {
			if ch.Next[j].visited {
				continue
			}
			stack = append(stack, ch.Next[j])
		}
	}
	for _, ch := range i.resBuf {
		ch.visited = false
	}
}

func (i *iterator) iterate(start *Change, f func(c *Change) (isContinue bool)) {
	if start == nil {
		return
	}
	// reset
	i.resBuf = i.resBuf[:0]
	i.stack = i.stack[:0]
	i.f = f

	i.topSort(start)
	for idx := len(i.resBuf) - 1; idx >= 0; idx-- {
		if !f(i.resBuf[idx]) {
			return
		}
	}
}

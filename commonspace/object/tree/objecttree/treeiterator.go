package objecttree

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

func (i *iterator) iterateSkip(start *Change, skipBefore *Change, include bool, f func(c *Change) (isContinue bool)) {
	skipping := true
	i.iterate(start, func(c *Change) (isContinue bool) {
		if skipping {
			if c == skipBefore {
				skipping = false
				if include {
					return f(c)
				}
			}
			return true
		}
		return f(c)
	})
}

func (i *iterator) topSort(start *Change) {
	stack := i.stack
	stack = append(stack, start)

	for len(stack) > 0 {
		ch := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		// here we visit the change second time to add it to results
		// all next changes at this point were visited
		if ch.branchesFinished {
			i.resBuf = append(i.resBuf, ch)
			ch.branchesFinished = false
			continue
		}

		if ch.visited {
			continue
		}

		// put the change again into stack, so we can add it to results
		// after all the next changes
		stack = append(stack, ch)
		ch.visited = true
		ch.branchesFinished = true

		for j := 0; j < len(ch.Next); j++ {
			if !ch.Next[j].visited {
				stack = append(stack, ch.Next[j])
			}
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

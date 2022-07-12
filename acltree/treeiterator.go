package acltree

import "sync"

var itPool = &sync.Pool{
	New: func() interface{} {
		return &iterator{}
	},
}

func newIterator() *iterator {
	return itPool.Get().(*iterator)
}

func freeIterator(i *iterator) {
	itPool.Put(i)
}

type iterator struct {
	compBuf    []*Change
	queue      []*Change
	doneMap    map[*Change]struct{}
	breakpoint *Change
	f          func(c *Change) bool
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

func (i *iterator) iterate(start *Change, f func(c *Change) (isContinue bool)) {
	if start == nil {
		return
	}
	// reset
	i.queue = i.queue[:0]
	i.compBuf = i.compBuf[:0]
	i.doneMap = make(map[*Change]struct{})
	i.queue = append(i.queue, start)
	i.breakpoint = nil
	i.f = f

	for len(i.queue) > 0 {
		c := i.queue[0]
		i.queue = i.queue[1:]
		nl := len(c.Next)
		if nl == 1 {
			if !i.iterateLin(c) {
				return
			}
			if i.breakpoint != nil {
				i.toQueue(i.breakpoint)
				i.breakpoint = nil
			}
		} else {
			_, done := i.doneMap[c]
			if !done {
				if !f(c) {
					return
				}
				i.doneMap[c] = struct{}{}
			}
			if nl != 0 {
				for _, next := range c.Next {
					i.toQueue(next)
				}
			}
		}
	}
}

func (i *iterator) iterateLin(c *Change) bool {
	for len(c.Next) == 1 {
		_, done := i.doneMap[c]
		if !done {
			if !i.f(c) {
				return false
			}
			i.doneMap[c] = struct{}{}
		}

		c = c.Next[0]
		if len(c.PreviousIds) > 1 {
			break
		}
	}
	if len(c.Next) == 0 && len(c.PreviousIds) <= 1 {
		if !i.f(c) {
			return false
		}
		i.doneMap[c] = struct{}{}
	} else {
		i.breakpoint = c
	}

	return true
}

func (i *iterator) comp(c1, c2 *Change) uint8 {
	if c1.Id == c2.Id {
		return 0
	}
	i.compBuf = i.compBuf[:0]
	i.compBuf = append(i.compBuf, c1.Next...)
	var uniq = make(map[*Change]struct{})
	var appendUniqueToBuf = func(next []*Change) {
		for _, n := range next {
			if _, ok := uniq[n]; !ok {
				i.compBuf = append(i.compBuf, n)
				uniq[n] = struct{}{}
			}
		}
	}
	var used int
	for len(i.compBuf)-used > 0 {
		l := len(i.compBuf) - used
		for _, n := range i.compBuf[used:] {
			delete(uniq, n)
			if n.Id == c2.Id {
				return 1
			} else {
				appendUniqueToBuf(n.Next)
			}
		}
		used += l
	}
	return 2
}

func (i *iterator) toQueue(c *Change) {
	var pos = -1
For:
	for idx, qc := range i.queue {
		switch i.comp(c, qc) {
		// exists
		case 0:
			return
		//
		case 1:
			pos = idx
			break For
		}
	}
	if pos == -1 {
		i.queue = append(i.queue, c)
	} else if pos == 0 {
		i.queue = append([]*Change{c}, i.queue...)
	} else {
		i.queue = append(i.queue[:pos], append([]*Change{c}, i.queue[pos:]...)...)
	}
}

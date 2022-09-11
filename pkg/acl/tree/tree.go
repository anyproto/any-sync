package tree

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"sort"
)

type Mode int

const (
	Append Mode = iota
	Rebuild
	Nothing
)

type Tree struct {
	root               *Change
	headIds            []string
	lastIteratedHeadId string
	metaHeadIds        []string
	attached           map[string]*Change
	unAttached         map[string]*Change
	// missed id -> list of dependency ids
	waitList       map[string][]string
	invalidChanges map[string]struct{}
	possibleRoots  []*Change

	// bufs
	visitedBuf []*Change
	stackBuf   []*Change
	addedBuf   []*Change

	duplicateEvents int
}

func (t *Tree) RootId() string {
	if t.root != nil {
		return t.root.Id
	}
	return ""
}

func (t *Tree) Root() *Change {
	return t.root
}

func (t *Tree) AddFast(changes ...*Change) []*Change {
	t.addedBuf = t.addedBuf[:0]
	for _, c := range changes {
		// ignore existing
		if _, ok := t.attached[c.Id]; ok {
			continue
		} else if _, ok := t.unAttached[c.Id]; ok {
			continue
		}
		t.add(c)
	}
	t.updateHeads()
	return t.addedBuf
}

func (t *Tree) AddMergedHead(c *Change) error {
	// check that it was not inserted previously
	if _, ok := t.attached[c.Id]; ok {
		return fmt.Errorf("change already exists") // TODO: named error
	} else if _, ok := t.unAttached[c.Id]; ok {
		return fmt.Errorf("change already exists")
	}

	// check that it was attached after adding
	if !t.add(c) {
		return fmt.Errorf("change is not attached")
	}

	// check that previous heads have new change as next
	for _, prevHead := range t.headIds {
		head := t.attached[prevHead]
		if len(head.Next) != 1 || head.Next[0].Id != c.Id {
			return fmt.Errorf("this is not a new head")
		}
	}
	t.headIds = []string{c.Id}
	return nil
}

func (t *Tree) Add(changes ...*Change) (mode Mode, added []*Change) {
	t.addedBuf = t.addedBuf[:0]
	var (
		// this is previous head id which should have been iterated last
		lastIteratedHeadId = t.lastIteratedHeadId
		empty              = t.Len() == 0
	)
	for _, c := range changes {
		// ignore existing
		if _, ok := t.attached[c.Id]; ok {
			continue
		} else if _, ok := t.unAttached[c.Id]; ok {
			continue
		}
		t.add(c)
	}
	if len(t.addedBuf) == 0 {
		mode = Nothing
		return
	}
	t.updateHeads()
	added = t.addedBuf

	if empty {
		mode = Rebuild
		return
	}

	// mode is Append for cases when we can safely start iterating from lastIteratedHeadId to build state
	// the idea here is that if all new changes have lastIteratedHeadId as previous,
	// then according to topological sorting order they will be looked at later than lastIteratedHeadId
	//
	// one important consideration is that if some unattached changes were added to the tree
	// as a result of adding new changes, then each of these unattached changes
	// will also have at least one of new changes as ancestor
	// and that means they will also be iterated later than lastIteratedHeadId
	mode = Append
	t.dfsNext([]*Change{t.attached[lastIteratedHeadId]},
		func(_ *Change) (isContinue bool) {
			return true
		},
		func(_ []*Change) {
			// checking if some new changes were not visited
			for _, ch := range changes {
				// if the change was not added to the tree, then skipping
				if _, ok := t.attached[ch.Id]; !ok {
					continue
				}
				// if some new change was not visited,
				// then we can't start from lastIteratedHeadId,
				// we need to start from root, so Rebuild
				if !ch.visited {
					mode = Rebuild
					break
				}
			}
		})

	return
}

// RemoveInvalidChange removes all the changes that are descendants of id
func (t *Tree) RemoveInvalidChange(id string) {
	stack := []string{id}
	// removing all children of this id (either next or unattached)
	for len(stack) > 0 {
		var exists bool
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if _, exists = t.invalidChanges[top]; exists {
			continue
		}

		var rem *Change
		t.invalidChanges[top] = struct{}{}
		if rem, exists = t.unAttached[top]; exists {
			delete(t.unAttached, top)
			// TODO: delete waitlist, this can only help for memory/performance
		} else if rem, exists = t.attached[top]; exists {
			// remove from all prev changes
			for _, id := range rem.PreviousIds {
				prev, exists := t.attached[id]
				if !exists {
					continue
				}
				for i, next := range prev.Next {
					if next.Id == top {
						prev.Next[i] = nil
						prev.Next = append(prev.Next[:i], prev.Next[i+1:]...)
						break
					}
				}
			}
			delete(t.attached, top)
		}
		for _, el := range rem.Next {
			stack = append(stack, el.Id)
		}
	}
	t.updateHeads()
}

func (t *Tree) add(c *Change) (attached bool) {
	if c == nil {
		return false
	}
	if _, exists := t.invalidChanges[c.Id]; exists {
		return false
	}

	if t.root == nil { // first element
		t.root = c
		t.lastIteratedHeadId = t.root.Id
		t.attached = map[string]*Change{
			c.Id: c,
		}
		t.unAttached = make(map[string]*Change)
		t.waitList = make(map[string][]string)
		t.invalidChanges = make(map[string]struct{})
		t.possibleRoots = make([]*Change, 0, 10)
		t.addedBuf = append(t.addedBuf, c)
		return true
	}
	if len(c.PreviousIds) > 1 {
		sort.Strings(c.PreviousIds)
	}
	// attaching only if all prev ids are attached
	attached = true
	for _, pid := range c.PreviousIds {
		if _, ok := t.attached[pid]; ok {
			continue
		}
		attached = false
		// updating wait list for either unseen or unAttached changes
		wl := t.waitList[pid]
		wl = append(wl, c.Id)
		t.waitList[pid] = wl
	}
	if attached {
		t.attach(c, true)
	} else {
		t.unAttached[c.Id] = c
	}
	return
}

func (t *Tree) canAttach(c *Change) (attach bool) {
	if c == nil {
		return false
	}
	attach = true
	for _, id := range c.PreviousIds {
		if _, exists := t.attached[id]; !exists {
			attach = false
			break
		}
	}
	return
}

func (t *Tree) attach(c *Change, newEl bool) {
	t.attached[c.Id] = c
	t.addedBuf = append(t.addedBuf, c)
	if !newEl {
		delete(t.unAttached, c.Id)
	}
	if c.IsSnapshot {
		t.possibleRoots = append(t.possibleRoots, c)
	}

	// add next to all prev changes
	for _, id := range c.PreviousIds {
		// prev id must already be attached if we attach this id, so we don't need to check if it exists
		prev := t.attached[id]
		// appending c to next changes of all previous changes
		if len(prev.Next) == 0 || prev.Next[len(prev.Next)-1].Id <= c.Id {
			prev.Next = append(prev.Next, c)
		} else {
			// inserting in correct position, before the change which is greater or equal
			insertIdx := 0
			for idx, el := range prev.Next {
				if el.Id >= c.Id {
					insertIdx = idx
					break
				}
			}
			prev.Next = append(prev.Next[:insertIdx+1], prev.Next[insertIdx:]...)
			prev.Next[insertIdx] = c
		}
	}
	// TODO: as a future optimization we can actually sort next later after we finished building the tree

	// clearing wait list
	if waitIds, ok := t.waitList[c.Id]; ok {
		for _, wid := range waitIds {
			// next can only be in unAttached, because if next is attached then previous (we) are attached
			// which is obviously not true, because we are attaching previous only now
			next := t.unAttached[wid]
			if t.canAttach(next) {
				t.attach(next, false)
			}
			// if we can't attach next that means that some other change will trigger attachment later,
			// so we don't care about those changes
		}
		delete(t.waitList, c.Id)
	}
}

func (t *Tree) after(id1, id2 string) (found bool) {
	t.iterate(t.attached[id2], func(c *Change) (isContinue bool) {
		if c.Id == id1 {
			found = true
			return false
		}
		return true
	})
	return
}

func (t *Tree) dfsPrev(stack []*Change, breakpoints []string, visit func(ch *Change) (isContinue bool), afterVisit func([]*Change)) {
	t.visitedBuf = t.visitedBuf[:0]

	// setting breakpoints as visited
	for _, breakpoint := range breakpoints {
		if ch, ok := t.attached[breakpoint]; ok {
			ch.visited = true
			t.visitedBuf = append(t.visitedBuf, ch)
		}
	}

	defer func() {
		if afterVisit != nil {
			afterVisit(t.visitedBuf)
		}
		for _, ch := range t.visitedBuf {
			ch.visited = false
		}
	}()

	for len(stack) > 0 {
		ch := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if ch.visited {
			continue
		}

		ch.visited = true
		t.visitedBuf = append(t.visitedBuf, ch)

		for _, prevId := range ch.PreviousIds {
			prevCh := t.attached[prevId]
			if !prevCh.visited {
				stack = append(stack, prevCh)
			}
		}
		if !visit(ch) {
			return
		}
	}
}

func (t *Tree) dfsNext(stack []*Change, visit func(ch *Change) (isContinue bool), afterVisit func([]*Change)) {
	t.visitedBuf = t.visitedBuf[:0]

	defer func() {
		if afterVisit != nil {
			afterVisit(t.visitedBuf)
		}
		for _, ch := range t.visitedBuf {
			ch.visited = false
		}
	}()

	for len(stack) > 0 {
		ch := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if ch.visited {
			continue
		}

		ch.visited = true
		t.visitedBuf = append(t.visitedBuf, ch)

		for _, next := range ch.Next {
			if !next.visited {
				stack = append(stack, next)
			}
		}
		if !visit(ch) {
			return
		}
	}
}

func (t *Tree) updateHeads() {
	var newHeadIds []string
	t.iterate(t.root, func(c *Change) (isContinue bool) {
		if len(c.Next) == 0 {
			newHeadIds = append(newHeadIds, c.Id)
		}
		return true
	})
	t.headIds = newHeadIds
	// the lastIteratedHeadId is the id of the head which was iterated last according to the order
	t.lastIteratedHeadId = newHeadIds[len(newHeadIds)-1]
	// TODO: check why do we need sorting here
	sort.Strings(t.headIds)
}

func (t *Tree) iterate(start *Change, f func(c *Change) (isContinue bool)) {
	it := newIterator()
	defer freeIterator(it)
	it.iterate(start, f)
}

func (t *Tree) iterateSkip(start *Change, skipBefore *Change, f func(c *Change) (isContinue bool)) {
	it := newIterator()
	defer freeIterator(it)
	it.iterateSkip(start, skipBefore, f)
}

func (t *Tree) IterateSkip(startId string, skipBeforeId string, f func(c *Change) (isContinue bool)) {
	it := newIterator()
	defer freeIterator(it)
	it.iterateSkip(t.attached[startId], t.attached[skipBeforeId], f)
}

func (t *Tree) Iterate(startId string, f func(c *Change) (isContinue bool)) {
	t.iterate(t.attached[startId], f)
}

func (t *Tree) IterateBranching(startId string, f func(c *Change, branchLevel int) (isContinue bool)) {
	// branchLevel indicates the number of parallel branches
	var bc int
	t.iterate(t.attached[startId], func(c *Change) (isContinue bool) {
		if pl := len(c.PreviousIds); pl > 1 {
			bc -= pl - 1
		}
		bl := bc
		if nl := len(c.Next); nl > 1 {
			bc += nl - 1
		}
		return f(c, bl)
	})
}

func (t *Tree) Hash() string {
	h := md5.New()
	n := 0
	t.iterate(t.root, func(c *Change) (isContinue bool) {
		n++
		fmt.Fprintf(h, "-%s", c.Id)
		return true
	})
	return fmt.Sprintf("%d-%x", n, h.Sum(nil))
}

func (t *Tree) GetDuplicateEvents() int {
	return t.duplicateEvents
}

func (t *Tree) ResetDuplicateEvents() {
	t.duplicateEvents = 0
}

func (t *Tree) Len() int {
	return len(t.attached)
}

func (t *Tree) Heads() []string {
	return t.headIds
}

func (t *Tree) HeadsChanges() []*Change {
	var heads []*Change
	for _, head := range t.headIds {
		heads = append(heads, t.attached[head])
	}
	return heads
}

func (t *Tree) String() string {
	var buf = bytes.NewBuffer(nil)
	t.Iterate(t.RootId(), func(c *Change) (isContinue bool) {
		buf.WriteString(c.Id)
		if len(c.Next) > 1 {
			buf.WriteString("-<")
		} else if len(c.Next) > 0 {
			buf.WriteString("->")
		} else {
			buf.WriteString("-|")
		}
		return true
	})
	return buf.String()
}

func (t *Tree) Get(id string) *Change {
	return t.attached[id]
}

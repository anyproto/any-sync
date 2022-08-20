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
	root        *Change
	headIds     []string
	metaHeadIds []string
	attached    map[string]*Change
	unAttached  map[string]*Change
	// missed id -> list of dependency ids
	waitList       map[string][]string
	invalidChanges map[string]struct{}

	// bufs
	iterCompBuf []*Change
	iterQueue   []*Change

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

func (t *Tree) AddFast(changes ...*Change) {
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
}

func (t *Tree) Add(changes ...*Change) (mode Mode) {
	var beforeHeadIds = t.headIds
	var attached bool
	var empty = t.Len() == 0
	for _, c := range changes {
		// ignore existing
		if _, ok := t.attached[c.Id]; ok {
			continue
		} else if _, ok := t.unAttached[c.Id]; ok {
			continue
		}
		if t.add(c) {
			attached = true
		}
	}
	if !attached {
		return Nothing
	}
	t.updateHeads()
	if empty {
		return Rebuild
	}
	for _, hid := range beforeHeadIds {
		for _, newCh := range changes {
			if _, ok := t.attached[newCh.Id]; ok {
				if !t.after(newCh.Id, hid) {
					return Rebuild
				}
			}
		}
	}
	return Append
}

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
		t.attached = map[string]*Change{
			c.Id: c,
		}
		t.unAttached = make(map[string]*Change)
		t.waitList = make(map[string][]string)
		t.invalidChanges = make(map[string]struct{})
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
	if !newEl {
		delete(t.unAttached, c.Id)
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

func (t *Tree) dfs(startChange string) (uniqMap map[string]*Change) {
	stack := make([]*Change, 0, 10)
	stack = append(stack, t.attached[startChange])
	uniqMap = map[string]*Change{}

	for len(stack) > 0 {
		ch := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, exists := uniqMap[ch.Id]; exists {
			continue
		}

		uniqMap[ch.Id] = ch

		for _, prev := range ch.PreviousIds {
			stack = append(stack, t.attached[prev])
		}
	}
	return uniqMap
}

func (t *Tree) updateHeads() {
	var newHeadIds, newMetaHeadIds []string
	t.iterate(t.root, func(c *Change) (isContinue bool) {
		if len(c.Next) == 0 {
			newHeadIds = append(newHeadIds, c.Id)
		}
		return true
	})
	t.headIds = newHeadIds
	t.metaHeadIds = newMetaHeadIds
	sort.Strings(t.headIds)
	sort.Strings(t.metaHeadIds)
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

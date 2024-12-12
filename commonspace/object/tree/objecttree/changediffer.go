package objecttree

type changeDiffer struct {
	toFind      map[string][]*Change
	headChanges []Change
	attached    map[string]*Change
	unAttached  map[string]*Change
	waitList    map[string][]string
}

func newChangeDiffer() *changeDiffer {
	return &changeDiffer{
		toFind:     make(map[string][]*Change),
		attached:   make(map[string]*Change),
		unAttached: make(map[string]*Change),
		waitList:   make(map[string][]string),
	}
}

func (d *changeDiffer) MustAttach(ch *Change) {
	d.attached[ch.Id] = ch
}

func (d *changeDiffer) AddChanges(changes []*Change) error {
	
}

func (d *changeDiffer) canAttach(c *Change) (attach bool) {
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

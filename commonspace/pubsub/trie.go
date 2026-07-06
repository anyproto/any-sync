package pubsub

// patternTrie is a per-space interest trie over '/'-separated topic segments,
// mirroring the NATS sublist shape: one level per segment, a literal-child map plus
// dedicated single-segment ('*') and tail ('>') wildcard slots per level. Terminals
// hold a refcount of subscribing streams so identical patterns from many streams
// share one node. Not goroutine-safe; the engine serializes access.
type patternTrie struct {
	root *trieLevel
	size int // number of live distinct patterns
}

type trieLevel struct {
	nodes map[string]*trieNode
	pwc   *trieNode // '*'
	fwc   *trieNode // '>'
}

type trieNode struct {
	next    *trieLevel
	pattern string // set iff this node terminates a pattern
	refs    int    // subscriber refcount for the terminated pattern
}

func newPatternTrie() *patternTrie {
	return &patternTrie{root: &trieLevel{}}
}

func (t *patternTrie) Len() int {
	return t.size
}

// Add registers one subscriber reference for the pattern (assumed validated).
// Returns true when the pattern is new to the trie (0 -> 1 transition).
func (t *patternTrie) Add(pattern string) bool {
	segs := splitTopic(pattern)
	level := t.root
	var node *trieNode
	for _, seg := range segs {
		node = level.child(seg)
		if node == nil {
			node = &trieNode{}
			level.setChild(seg, node)
		}
		if node.next == nil {
			node.next = &trieLevel{}
		}
		level = node.next
	}
	node.pattern = pattern
	node.refs++
	if node.refs == 1 {
		t.size++
		return true
	}
	return false
}

// Remove drops one subscriber reference; on the last reference the pattern is pruned.
// Returns true when the pattern was removed entirely (1 -> 0 transition).
func (t *patternTrie) Remove(pattern string) bool {
	segs := splitTopic(pattern)
	return t.remove(t.root, segs, pattern)
}

func (t *patternTrie) remove(level *trieLevel, segs []string, pattern string) bool {
	if len(segs) == 0 {
		return false
	}
	node := level.child(segs[0])
	if node == nil {
		return false
	}
	var removed bool
	if len(segs) == 1 {
		if node.refs == 0 {
			return false
		}
		node.refs--
		if node.refs > 0 {
			return false
		}
		node.pattern = ""
		t.size--
		removed = true
	} else {
		if node.next == nil {
			return false
		}
		removed = t.remove(node.next, segs[1:], pattern)
	}
	if node.refs == 0 && (node.next == nil || node.next.empty()) {
		level.deleteChild(segs[0])
	}
	return removed
}

// Match walks the trie with the segments of a fully-qualified topic and appends
// every matching pattern to dst. Match order follows NATS matchLevel: the
// tail-wildcard terminal is collected at each level (it matches one-or-more
// remaining segments), then the '*' branch, then the literal branch.
func (t *patternTrie) Match(topic string, dst []string) []string {
	segs := splitTopic(topic)
	return matchLevel(t.root, segs, dst)
}

func matchLevel(level *trieLevel, segs []string, dst []string) []string {
	if level == nil || len(segs) == 0 {
		return dst
	}
	if level.fwc != nil && level.fwc.refs > 0 {
		dst = append(dst, level.fwc.pattern)
	}
	if level.pwc != nil {
		dst = matchNode(level.pwc, segs[1:], dst)
	}
	if node := level.literal(segs[0]); node != nil {
		dst = matchNode(node, segs[1:], dst)
	}
	return dst
}

// matchNode resolves a node that consumed one segment against the remaining ones.
func matchNode(node *trieNode, rest []string, dst []string) []string {
	if len(rest) == 0 {
		if node.refs > 0 {
			dst = append(dst, node.pattern)
		}
		return dst
	}
	return matchLevel(node.next, rest, dst)
}

func (l *trieLevel) child(seg string) *trieNode {
	switch seg {
	case wildcardOne:
		return l.pwc
	case wildcardTail:
		return l.fwc
	default:
		return l.nodes[seg]
	}
}

func (l *trieLevel) literal(seg string) *trieNode {
	return l.nodes[seg]
}

func (l *trieLevel) setChild(seg string, n *trieNode) {
	switch seg {
	case wildcardOne:
		l.pwc = n
	case wildcardTail:
		l.fwc = n
	default:
		if l.nodes == nil {
			l.nodes = make(map[string]*trieNode)
		}
		l.nodes[seg] = n
	}
}

func (l *trieLevel) deleteChild(seg string) {
	switch seg {
	case wildcardOne:
		l.pwc = nil
	case wildcardTail:
		l.fwc = nil
	default:
		delete(l.nodes, seg)
	}
}

func (l *trieLevel) empty() bool {
	return len(l.nodes) == 0 && l.pwc == nil && l.fwc == nil
}

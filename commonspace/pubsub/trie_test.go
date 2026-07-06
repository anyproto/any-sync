package pubsub

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func match(t *patternTrie, topic string) []string {
	res := t.Match(topic, nil)
	sort.Strings(res)
	return res
}

func TestTrieExactMatch(t *testing.T) {
	tr := newPatternTrie()
	require.True(t, tr.Add("presence"))
	require.True(t, tr.Add("chat/abc/typing"))

	require.Equal(t, []string{"presence"}, match(tr, "presence"))
	require.Equal(t, []string{"chat/abc/typing"}, match(tr, "chat/abc/typing"))
	require.Empty(t, match(tr, "chat/abc"))
	require.Empty(t, match(tr, "chat/abc/typing/extra"))
	require.Empty(t, match(tr, "other"))
}

func TestTrieSingleSegmentWildcard(t *testing.T) {
	tr := newPatternTrie()
	tr.Add("chat/*/typing")

	require.Equal(t, []string{"chat/*/typing"}, match(tr, "chat/abc/typing"))
	require.Empty(t, match(tr, "chat/typing"))          // '*' must consume one segment
	require.Empty(t, match(tr, "chat/a/b/typing"))      // '*' consumes exactly one
	require.Empty(t, match(tr, "chat/abc/typing/late")) // trailing extra segment

	tr.Add("*")
	require.Equal(t, []string{"*"}, match(tr, "presence"))
	require.Empty(t, match(tr, "a/b"))
}

func TestTrieTailWildcard(t *testing.T) {
	tr := newPatternTrie()
	tr.Add("chat/>")

	require.Equal(t, []string{"chat/>"}, match(tr, "chat/abc"))
	require.Equal(t, []string{"chat/>"}, match(tr, "chat/a/b/c"))
	require.Empty(t, match(tr, "chat")) // '>' requires at least one segment

	tr.Add(">")
	require.Equal(t, []string{">"}, match(tr, "anything"))
	require.Equal(t, []string{">", "chat/>"}, match(tr, "chat/x"))
}

func TestTrieOverlappingPatterns(t *testing.T) {
	tr := newPatternTrie()
	tr.Add("chat/>")
	tr.Add("chat/*/typing")
	tr.Add("chat/abc/typing")

	require.Equal(t,
		[]string{"chat/*/typing", "chat/>", "chat/abc/typing"},
		match(tr, "chat/abc/typing"))
	require.Equal(t, []string{"chat/>"}, match(tr, "chat/abc/presence"))
}

func TestTrieAccWildcard(t *testing.T) {
	tr := newPatternTrie()
	tr.Add("acc/online/*")
	require.Equal(t, []string{"acc/online/*"}, match(tr, "acc/online/A1"))
	require.Equal(t, []string{"acc/online/*"}, match(tr, "acc/online/A2"))
	require.Empty(t, match(tr, "acc/cursor/A1"))
}

func TestTrieRefcounting(t *testing.T) {
	tr := newPatternTrie()
	require.True(t, tr.Add("chat/>"))
	require.False(t, tr.Add("chat/>")) // second subscriber, same pattern
	require.Equal(t, 1, tr.Len())

	require.False(t, tr.Remove("chat/>")) // still one ref left
	require.Equal(t, []string{"chat/>"}, match(tr, "chat/x"))

	require.True(t, tr.Remove("chat/>")) // last ref gone
	require.Empty(t, match(tr, "chat/x"))
	require.Equal(t, 0, tr.Len())

	// removing a non-existent pattern is a no-op
	require.False(t, tr.Remove("chat/>"))
	require.False(t, tr.Remove("never/added"))
}

func TestTriePruning(t *testing.T) {
	tr := newPatternTrie()
	tr.Add("a/b/c/d")
	tr.Add("a/b/x")
	require.True(t, tr.Remove("a/b/c/d"))
	// sibling under the shared prefix still matches
	require.Equal(t, []string{"a/b/x"}, match(tr, "a/b/x"))
	require.Empty(t, match(tr, "a/b/c/d"))

	tr.Remove("a/b/x")
	require.True(t, tr.root.empty(), "trie should be fully pruned")
}

func TestTrieInteriorTerminal(t *testing.T) {
	tr := newPatternTrie()
	// a pattern that is a prefix of another
	tr.Add("chat")
	tr.Add("chat/abc")
	require.Equal(t, []string{"chat"}, match(tr, "chat"))
	require.Equal(t, []string{"chat/abc"}, match(tr, "chat/abc"))

	// removing the prefix pattern keeps the longer one intact
	require.True(t, tr.Remove("chat"))
	require.Empty(t, match(tr, "chat"))
	require.Equal(t, []string{"chat/abc"}, match(tr, "chat/abc"))
}

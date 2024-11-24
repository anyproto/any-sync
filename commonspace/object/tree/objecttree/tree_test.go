package objecttree

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
)

func newChange(id string, snapshotId string, prevIds ...string) *Change {
	return &Change{
		PreviousIds: prevIds,
		Id:          id,
		SnapshotId:  snapshotId,
		IsSnapshot:  false,
	}
}

func newSnapshot(id, snapshotId string, prevIds ...string) *Change {
	return &Change{
		PreviousIds: prevIds,
		Id:          id,
		SnapshotId:  snapshotId,
		IsSnapshot:  true,
	}
}

func TestTree_AddMergedHead(t *testing.T) {
	tr := new(Tree)
	_, _ = tr.Add(
		newSnapshot("root", ""),
		newChange("one", "root", "root"),
	)
	require.Equal(t, tr.lastIteratedHeadId, "one")
	tr.AddMergedHead(newChange("two", "root", "one"))
	require.Equal(t, tr.lastIteratedHeadId, "two")
}

func TestTree_Add(t *testing.T) {
	t.Run("add first el", func(t *testing.T) {
		tr := new(Tree)
		res, _ := tr.Add(newSnapshot("root", ""))
		assert.Equal(t, Rebuild, res)
		assert.Equal(t, tr.root.Id, "root")
		assert.Equal(t, []string{"root"}, tr.Heads())
	})
	t.Run("linear add", func(t *testing.T) {
		tr := new(Tree)
		res, _ := tr.Add(
			newSnapshot("root", ""),
			newChange("one", "root", "root"),
			newChange("two", "root", "one"),
		)
		assert.Equal(t, Rebuild, res)
		assert.Equal(t, []string{"two"}, tr.Heads())
		res, _ = tr.Add(newChange("three", "root", "two"))
		assert.Equal(t, Append, res)
		el := tr.root
		var ids []string
		for el != nil {
			ids = append(ids, el.Id)
			if len(el.Next) > 0 {
				el = el.Next[0]
			} else {
				el = nil
			}
		}
		assert.Equal(t, []string{"root", "one", "two", "three"}, ids)
		assert.Equal(t, []string{"three"}, tr.Heads())
	})
	t.Run("branch", func(t *testing.T) {
		tr := new(Tree)
		res, _ := tr.Add(
			newSnapshot("root", ""),
			newChange("1", "root", "root"),
			newChange("2", "root", "1"),
		)
		assert.Equal(t, Rebuild, res)
		assert.Equal(t, []string{"2"}, tr.Heads())
		res, _ = tr.Add(
			newChange("1.2", "root", "1.1"),
			newChange("1.3", "root", "1.2"),
			newChange("1.1", "root", "1"),
		)
		assert.Equal(t, Rebuild, res)
		assert.Len(t, tr.attached["1"].Next, 2)
		assert.Len(t, tr.unAttached, 0)
		assert.Len(t, tr.attached, 6)
		assert.Equal(t, []string{"1.3", "2"}, tr.Heads())
	})
	t.Run("branch union", func(t *testing.T) {
		tr := new(Tree)
		res, _ := tr.Add(
			newSnapshot("root", ""),
			newChange("1", "root", "root"),
			newChange("2", "root", "1"),
			newChange("1.2", "root", "1.1"),
			newChange("1.3", "root", "1.2"),
			newChange("1.1", "root", "1"),
			newChange("3", "root", "2", "1.3"),
			newChange("4", "root", "3"),
		)
		assert.Equal(t, Rebuild, res)
		assert.Len(t, tr.unAttached, 0)
		assert.Len(t, tr.attached, 8)
		assert.Equal(t, []string{"4"}, tr.Heads())
	})
	t.Run("big set", func(t *testing.T) {
		tr := new(Tree)
		tr.Add(newSnapshot("root", ""))
		var changes []*Change
		for i := 0; i < 10000; i++ {
			if i == 0 {
				changes = append(changes, newChange(fmt.Sprint(i), "root", "root"))
			} else {
				changes = append(changes, newChange(fmt.Sprint(i), "root", fmt.Sprint(i-1)))
			}
		}
		st := time.Now()
		tr.AddFast(changes...)
		t.Log(time.Since(st))
		assert.Equal(t, []string{"9999"}, tr.Heads())
	})
}

func TestTree_Hash(t *testing.T) {
	tr := new(Tree)
	tr.Add(newSnapshot("root", ""))
	hash1 := tr.Hash()
	assert.Equal(t, tr.Hash(), hash1)
	tr.Add(newChange("1", "root", "root"))
	assert.NotEqual(t, tr.Hash(), hash1)
	assert.Equal(t, tr.Hash(), tr.Hash())
}

func TestTree_AddFuzzy(t *testing.T) {
	rand.Seed(time.Now().Unix())
	getChanges := func() []*Change {
		changes := []*Change{
			newChange("1", "root", "root"),
			newChange("2", "root", "1"),
			newChange("1.2", "root", "1.1"),
			newChange("1.3", "root", "1.2"),
			newChange("1.1", "root", "1"),
			newChange("3", "root", "2", "1.3"),
		}
		rand.Shuffle(len(changes), func(i, j int) {
			changes[i], changes[j] = changes[j], changes[i]
		})
		return changes
	}
	var phash string
	for i := 0; i < 100; i++ {
		tr := new(Tree)
		tr.Add(newSnapshot("root", ""))
		tr.Add(getChanges()...)
		assert.Len(t, tr.unAttached, 0)
		assert.Len(t, tr.attached, 7)
		hash := tr.Hash()
		if phash != "" {
			assert.Equal(t, phash, hash)
		}
		phash = hash
		assert.Equal(t, []string{"3"}, tr.Heads())
	}
}

func TestTree_CountersReduce(t *testing.T) {
	t.Run("reduce once", func(t *testing.T) {
		tr := new(Tree)
		tr.Add(
			newSnapshot("0", ""),
			newChange("1", "0", "0"),
			newChange("1.1", "0", "1"),
			newChange("1.2", "0", "1"),
			newChange("1.4", "0", "1.2"),
			newChange("1.3", "0", "1"),
			newChange("1.3.1", "0", "1.3"),
			newChange("1.2+3", "0", "1.4", "1.3.1"),
			newChange("1.2+3.1", "0", "1.2+3"),
			newSnapshot("10", "0", "1.2+3.1", "1.1"),
			newChange("last", "10", "10"),
		)
		t.Run("reduce", func(t *testing.T) {
			tr.reduceTree()
			assert.Equal(t, "10", tr.RootId())
			var res []string
			tr.IterateSkip(tr.RootId(), func(c *Change) (isContinue bool) {
				res = append(res, c.Id)
				return true
			})
			assert.Equal(t, []string{"10", "last"}, res)
			assert.Equal(t, 1, tr.attached["10"].SnapshotCounter)
		})
	})
	t.Run("snapshots in line", func(t *testing.T) {
		tr := new(Tree)
		tr.Add(
			newSnapshot("0", ""),
			newSnapshot("0.1", "0", "0"),
			newSnapshot("0.2", "0", "0"),
			newSnapshot("1", "0", "0.1", "0.2"),
			newSnapshot("2", "1", "1"),
			newSnapshot("3", "2", "2"),
		)
		t.Run("reduce", func(t *testing.T) {
			tr.reduceTree()
			assert.Equal(t, "3", tr.RootId())
			var res []string
			tr.IterateSkip(tr.RootId(), func(c *Change) (isContinue bool) {
				res = append(res, c.Id)
				return true
			})
			assert.Equal(t, []string{"3"}, res)
			assert.Equal(t, 3, tr.attached["3"].SnapshotCounter)
		})
	})
	t.Run("many snapshots", func(t *testing.T) {
		tr := new(Tree)
		tr.Add(
			newSnapshot("0", ""),
			newSnapshot("1", "0", "0"),
			newChange("1.2", "1", "1"),
			newChange("1.3", "1", "1"),
			newChange("1.3.1", "1", "1.3"),
			newSnapshot("1.2+3", "1", "1.2", "1.3.1"),
			newChange("1.2+3.1", "1.2+3", "1.2+3"),
			newChange("1.2+3.2", "1.2+3", "1.2+3"),
			newSnapshot("10", "1.2+3", "1.2+3.1", "1.2+3.2"),
			newChange("last", "10", "10"),
			newChange("last1", "10", "10"),
			newChange("last2", "1.2+3", "1.2+3"),
		)
		t.Run("reduce", func(t *testing.T) {
			tr.reduceTree()
			assert.Equal(t, "1.2+3", tr.RootId())
			var res []string
			tr.IterateSkip(tr.RootId(), func(c *Change) (isContinue bool) {
				res = append(res, c.Id)
				return true
			})
			assert.Equal(t, []string{"1.2+3", "1.2+3.1", "1.2+3.2", "10", "last", "last1", "last2"}, res)
			assert.Equal(t, 3, tr.attached["10"].SnapshotCounter)
			assert.Equal(t, 2, tr.attached["1.2+3"].SnapshotCounter)
		})
	})
}

func TestTree_Iterate(t *testing.T) {
	t.Run("complex tree", func(t *testing.T) {
		tr := new(Tree)
		tr.Add(
			newSnapshot("0", ""),
			newChange("1", "0", "0"),
			newChange("1.1", "0", "1"),
			newChange("1.2", "0", "1"),
			newChange("1.4", "0", "1.2"),
			newChange("1.3", "0", "1"),
			newChange("1.3.1", "0", "1.3"),
			newChange("1.2+3", "0", "1.4", "1.3.1"),
			newChange("1.2+3.1", "0", "1.2+3"),
			newChange("10", "0", "1.2+3.1", "1.1"),
			newChange("last", "0", "10"),
		)
		var res []string
		tr.IterateSkip("0", func(c *Change) (isContinue bool) {
			res = append(res, c.Id)
			return true
		})
		res = res[:0]
		tr.IterateSkip("0", func(c *Change) (isContinue bool) {
			res = append(res, c.Id)
			return true
		})
		assert.Equal(t, []string{"0", "1", "1.1", "1.2", "1.4", "1.3", "1.3.1", "1.2+3", "1.2+3.1", "10", "last"}, res)
	})
	t.Run("complex tree with diverging branches", func(t *testing.T) {
		tr := new(Tree)
		tr.Add(
			newSnapshot("0", ""),
			newChange("1", "0", "0"),
			newChange("2", "0", "0"),
			newChange("2.1", "0", "2"),
			newChange("2.2", "0", "2"),
			newChange("2.4", "0", "2.2"),
			newChange("2.3", "0", "2"),
			newChange("2.3.1", "0", "2.3"),
			newChange("2.2+3", "0", "2.4", "2.3.1"),
			newChange("2.2+3.1", "0", "2.2+3"),
			newChange("10", "0", "2.2+3.1", "2.1"),
			newChange("last", "0", "10"),
		)
		var res []string
		tr.IterateSkip("1", func(c *Change) (isContinue bool) {
			res = append(res, c.Id)
			return true
		})
		assert.Equal(t, []string{"1", "2", "2.1", "2.2", "2.4", "2.3", "2.3.1", "2.2+3", "2.2+3.1", "10", "last"}, res)
	})
}

func TestTree_Orders(t *testing.T) {
	t.Run("test orders", func(t *testing.T) {
		tr := new(Tree)
		tr.Add(
			newSnapshot("0", ""),
			newChange("1", "0", "0"),
			newChange("1.1", "0", "1"),
			newChange("1.2", "0", "1"),
			newChange("1.4", "0", "1.2"),
			newChange("1.3", "0", "1"),
			newChange("1.3.1", "0", "1.3"),
			newChange("1.2+3", "0", "1.4", "1.3.1"),
			newChange("1.2+3.1", "0", "1.2+3"),
			newChange("10", "0", "1.2+3.1", "1.1"),
			newChange("last", "0", "10"),
		)
		var res []string
		var changes []*Change
		tr.IterateSkip("0", func(c *Change) (isContinue bool) {
			res = append(res, c.Id)
			changes = append(changes, c)
			return true
		})
		slices.Reverse(changes)
		slices.SortFunc(changes, func(c1, c2 *Change) int {
			return strings.Compare(c1.OrderId, c2.OrderId)
		})
		require.Equal(t, len(changes), len(res))
		for i := 0; i < len(changes); i++ {
			require.Equal(t, changes[i].Id, res[i])
		}
	})
	t.Run("test orders split", func(t *testing.T) {
		tr := new(Tree)
		tr.Add(
			newSnapshot("0", ""),
			newChange("1", "0", "0"),
			newChange("1.1", "0", "1"),
			newChange("1.2", "0", "1"),
			newChange("1.4", "0", "1.2"),
		)
		tr.Add(
			newChange("1.3", "0", "1"),
			newChange("1.3.1", "0", "1.3"),
			newChange("1.2+3", "0", "1.4", "1.3.1"),
			newChange("1.2+3.1", "0", "1.2+3"),
			newChange("10", "0", "1.2+3.1", "1.1"),
			newChange("last", "0", "10"),
		)
		var res []string
		var changes []*Change
		tr.IterateSkip("0", func(c *Change) (isContinue bool) {
			res = append(res, c.Id)
			changes = append(changes, c)
			return true
		})
		slices.Reverse(changes)
		slices.SortFunc(changes, func(c1, c2 *Change) int {
			return strings.Compare(c1.OrderId, c2.OrderId)
		})
		require.Equal(t, len(changes), len(res))
		for i := 0; i < len(changes); i++ {
			require.Equal(t, changes[i].Id, res[i])
		}
	})
	t.Run("test orders gap", func(t *testing.T) {
		tr := new(Tree)
		tr.Add(
			newSnapshot("0", ""),
			newChange("1", "0", "0"),
			newChange("2", "0", "1"),
			newChange("3", "0", "2"),
		)
		tr.Add(
			newChange("0.1", "0", "0"),
			newChange("0.2", "0", "0.1"),
			newChange("0.3", "0", "0.2"),
		)
		var res []string
		var changes []*Change
		tr.IterateSkip("0", func(c *Change) (isContinue bool) {
			res = append(res, c.Id)
			changes = append(changes, c)
			return true
		})
		slices.SortFunc(changes, func(c1, c2 *Change) int {
			return strings.Compare(c1.OrderId, c2.OrderId)
		})
		require.Equal(t, len(changes), len(res))
		for i := 0; i < len(changes); i++ {
			require.Equal(t, changes[i].Id, res[i])
		}
	})
}

func BenchmarkTree_Add(b *testing.B) {
	getChanges := func() []*Change {
		return []*Change{
			newChange("1", "root", "root"),
			newChange("2", "root", "1"),
			newChange("1.2", "root", "1.1"),
			newChange("1.3", "root", "1.2"),
			newChange("1.1", "root", "1"),
			newChange("3", "root", "2", "1.3"),
		}
	}
	b.Run("by one", func(b *testing.B) {
		tr := new(Tree)
		tr.Add(newSnapshot("root", ""))
		tr.Add(getChanges()...)
		for i := 0; i < b.N; i++ {
			tr.Add(newChange(fmt.Sprint(i+4), "root", fmt.Sprint(i+3)))
		}
	})
	b.Run("add", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tr := new(Tree)
			tr.Add(newSnapshot("root", ""))
			tr.Add(getChanges()...)
		}
	})
	b.Run("add fast", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tr := new(Tree)
			tr.AddFast(newSnapshot("root", ""))
			tr.AddFast(getChanges()...)
		}
	})
}

func BenchmarkTree_IterateLinear(b *testing.B) {
	// prepare linear tree
	tr := new(Tree)
	tr.AddFast(newSnapshot("0", ""))
	for j := 0; j < 10000; j++ {
		tr.Add(newChange(fmt.Sprint(j+1), "0", fmt.Sprint(j)))
	}
	b.Run("add linear", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tr.IterateSkip("0", func(c *Change) (isContinue bool) {
				return true
			})
		}
	})
}

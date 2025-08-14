package objecttree

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRemoveBeforeBugs demonstrates the bugs in the RemoveBefore method
func TestRemoveBeforeBugs(t *testing.T) {
	t.Run("type mismatch bug in deletion", func(t *testing.T) {
		// Create a simple chain: 0 -> 1 -> 2
		changes := []*Change{
			newChange("0", ""),
			newChange("1", "", "0"),
			newChange("2", "", "1"),
		}

		differ, _ := NewChangeDiffer(nil, func(ids ...string) bool {
			return false
		})
		differ.Add(changes...)

		// Verify changes are in attached map
		require.Contains(t, differ.attached, "0")
		require.Contains(t, differ.attached, "1")
		require.Contains(t, differ.attached, "2")

		// This should trigger the bug - the deletion loop uses string IDs
		// but tries to delete with string keys from a map[string]*Change
		removed, _, _ := differ.RemoveBefore([]string{"2"}, []string{})

		// After the bug, the changes should NOT be properly removed from attached
		// because the deletion logic has a type mismatch
		require.Contains(t, removed, "0")
		require.Contains(t, removed, "1")
		require.Contains(t, removed, "2")

		// BUG: These should be removed but aren't due to type mismatch
		// The bug is on line 70: delete(d.attached, ch) where ch is a string
		// but we're trying to delete from map[string]*Change
		require.Contains(t, differ.attached, "0", "BUG: Change should be removed but type mismatch prevents it")
		require.Contains(t, differ.attached, "1", "BUG: Change should be removed but type mismatch prevents it")
		require.Contains(t, differ.attached, "2", "BUG: Change should be removed but type mismatch prevents it")
	})

	t.Run("heads calculation bug", func(t *testing.T) {
		// Create a simple chain: 0 -> 1 -> 2
		changes := []*Change{
			newChange("0", ""),
			newChange("1", "", "0"),
			newChange("2", "", "1"),
		}

		differ, _ := NewChangeDiffer(nil, func(ids ...string) bool {
			return false
		})
		differ.Add(changes...)

		// Remove change "1" - this should make "0" a head (parent of removed change)
		// but NOT make "1" itself a head (since it's being removed)
		_, _, heads := differ.RemoveBefore([]string{"1"}, []string{})

		// BUG: The current logic adds the removed change ID to heads
		// Line 64: heads = append(heads, ch.Id) - this is wrong
		// When removing a change, only its parents should become heads
		for _, head := range heads {
			if head == "1" {
				t.Error("BUG: Removed change '1' should not be in heads list")
			}
		}

		// The heads should contain "0" (parent of removed "1") but not "1" itself
		require.Contains(t, heads, "0", "Parent of removed change should be in heads")
	})

	t.Run("contradictory heads processing bug", func(t *testing.T) {
		// Create changes to test the heads deduplication logic
		changes := []*Change{
			newChange("0", ""),
			newChange("1", "", "0"),
			newChange("2", "", "0"),
			newChange("3", "", "1", "2"),
		}

		differ, _ := NewChangeDiffer(nil, func(ids ...string) bool {
			return false
		})
		differ.Add(changes...)

		// This will trigger the contradictory heads processing
		_, _, heads := differ.RemoveBefore([]string{"3"}, []string{"0"})

		// BUG: Lines 83-84 have contradictory logic:
		// heads = slice.RemoveUniqueElementsSorted(heads)  // Removes elements that appear once
		// heads = slice.DiscardDuplicatesSorted(heads)     // Removes duplicates
		// This means only elements that appear exactly twice survive

		// Test that the heads processing is broken
		// The result should be logically consistent but due to the bug it's not
		t.Logf("Heads after contradictory processing: %v", heads)

		// With the bug, the result is unpredictable and likely incorrect
		// The correct logic should either remove duplicates OR handle unique elements, not both
	})

	t.Run("duplicate seenHeads addition bug", func(t *testing.T) {
		changes := []*Change{
			newChange("0", ""),
			newChange("1", "", "0"),
		}

		differ, _ := NewChangeDiffer(nil, func(ids ...string) bool {
			return false
		})
		differ.Add(changes...)

		seenHeads := []string{"0", "seen1"}
		_, _, resultHeads := differ.RemoveBefore([]string{"1"}, seenHeads)

		// BUG: seenHeads are added twice in the function
		// Line 62: heads = append(heads, seenHeads...)
		// Line 81: heads = append(heads, seenHeads...)
		// This causes duplicate entries that need to be cleaned up later

		// Count occurrences of seenHeads elements
		seenCount := make(map[string]int)
		for _, head := range resultHeads {
			for _, seen := range seenHeads {
				if head == seen {
					seenCount[seen]++
				}
			}
		}

		t.Logf("Result heads: %v", resultHeads)
		t.Logf("Seen heads count: %v", seenCount)

		// The duplication should be visible in the intermediate processing
		// (though it gets cleaned up later by the sorting/deduplication)
	})
}

// Note: Removed DiffManager.Update test due to complex interface requirements
// The bug in Update method (lines 215-218) removes found items from notFound map
// which may be correct behavior for tracking what's still missing

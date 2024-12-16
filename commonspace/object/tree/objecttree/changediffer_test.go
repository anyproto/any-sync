package objecttree

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChangeDiffer_Add(t *testing.T) {
	t.Run("remove all", func(t *testing.T) {
		changes := []*Change{
			newChange("0", ""),
			newChange("1", "0", "0"),
			newChange("2", "0", "0"),
			newChange("3", "0", "1", "2"),
			newChange("4", "0", "0"),
			newChange("5", "0", "4"),
			newChange("6", "0", "5"),
			newChange("7", "0", "3", "6"),
		}
		differ := NewChangeDiffer(nil, func(ids ...string) bool {
			return false
		})
		differ.Add(changes...)
		res, notFound := differ.RemoveBefore([]string{"7"})
		require.Len(t, notFound, 0)
		require.Equal(t, len(changes), len(res))
	})
	t.Run("remove in two parts", func(t *testing.T) {
		changes := []*Change{
			newChange("0", ""),
			newChange("1", "0", "0"),
			newChange("2", "0", "0"),
			newChange("3", "0", "1", "2"),
			newChange("4", "0", "0"),
			newChange("5", "0", "4"),
			newChange("6", "0", "5"),
			newChange("7", "0", "3", "6"),
		}
		differ := NewChangeDiffer(nil, func(ids ...string) bool {
			return false
		})
		differ.Add(changes...)
		res, notFound := differ.RemoveBefore([]string{"4"})
		require.Len(t, notFound, 0)
		require.Equal(t, 2, len(res))
		res, notFound = differ.RemoveBefore([]string{"7"})
		require.Len(t, notFound, 0)
		require.Equal(t, 6, len(res))
	})
	t.Run("add and remove", func(t *testing.T) {
		changes := []*Change{
			newChange("0", ""),
			newChange("1", "0", "0"),
			newChange("2", "0", "0"),
			newChange("3", "0", "1", "2"),
		}
		differ := NewChangeDiffer(nil, func(ids ...string) bool {
			return false
		})
		differ.Add(changes...)
		res, notFound := differ.RemoveBefore([]string{"3"})
		require.Len(t, notFound, 0)
		require.Equal(t, len(changes), len(res))
		changes = []*Change{
			newChange("4", "0", "0"),
			newChange("5", "0", "4"),
			newChange("6", "0", "5"),
			newChange("7", "0", "3", "6"),
		}
		differ.Add(changes...)
		res, notFound = differ.RemoveBefore([]string{"7"})
		require.Len(t, notFound, 0)
		require.Equal(t, len(changes), len(res))
	})
	t.Run("remove not found", func(t *testing.T) {
		differ := NewChangeDiffer(nil, func(ids ...string) bool {
			return false
		})
		_, notFound := differ.RemoveBefore([]string{"3", "4", "5"})
		require.Len(t, notFound, 3)
	})
	t.Run("exists in storage", func(t *testing.T) {
		storedIds := []string{"3", "4", "5"}
		differ := NewChangeDiffer(nil, func(ids ...string) bool {
			for _, id := range ids {
				if !slices.Contains(storedIds, id) {
					return false
				}
			}
			return true
		})
		res, notFound := differ.RemoveBefore([]string{"3", "4", "5"})
		require.Len(t, res, 0)
		require.Len(t, notFound, 0)
	})
}

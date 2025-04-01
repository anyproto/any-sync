package headsync

import (
	"strings"
)

func concatStrings(strs []string) string {
	if len(strs) == 1 {
		return strs[0]
	}
	var (
		b        strings.Builder
		totalLen int
	)
	for _, s := range strs {
		totalLen += len(s)
	}

	b.Grow(totalLen)
	for _, s := range strs {
		b.WriteString(s)
	}
	return b.String()
}

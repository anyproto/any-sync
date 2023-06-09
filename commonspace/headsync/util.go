package headsync

import "strings"

func concatStrings(strs []string) string {
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

func splitString(str string) (res []string) {
	const cidLen = 59
	for i := 0; i < len(str); i += cidLen {
		res = append(res, str[i:i+cidLen])
	}
	return
}

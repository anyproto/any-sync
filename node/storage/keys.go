package storage

import (
	"bytes"
	"strings"
)

type treeKeys struct {
	id string
}

func (t treeKeys) HeadsKey() []byte {
	return joinStringsToBytes("t", t.id, "heads")
}

func (t treeKeys) RootKey() []byte {
	return joinStringsToBytes("t", t.id)
}

func (t treeKeys) RawChangeKey(id string) []byte {
	return joinStringsToBytes("t", t.id, id)
}

type spaceKeys struct {
}

var headerKey = []byte("header")
var aclKey = []byte("acl")

func (s spaceKeys) HeaderKey() []byte {
	return headerKey
}

func (s spaceKeys) ACLKey() []byte {
	return aclKey
}

func isRootKey(key string) bool {
	return strings.HasPrefix(key, "t/") && strings.Count(key, "/") == 2
}

func joinStringsToBytes(strs ...string) []byte {
	var (
		b        bytes.Buffer
		totalLen int
	)
	for _, s := range strs {
		totalLen += len(s)
	}
	// adding separators
	totalLen += len(strs) - 1
	b.Grow(totalLen)
	for idx, s := range strs {
		if idx > 0 {
			b.WriteString("/")
		}
		b.WriteString(s)
	}
	return b.Bytes()
}

package storage

import (
	"bytes"
	"strings"
)

type aclKeys struct {
}

var aclHeadIdKey = []byte("a/headId")
var aclRootIdKey = []byte("a/rootId")

func (a aclKeys) HeadIdKey() []byte {
	return aclHeadIdKey
}

func (a aclKeys) RootIdKey() []byte {
	return aclRootIdKey
}

func (a aclKeys) RawRecordKey(id string) []byte {
	return joinStringsToBytes("a", id)
}

type treeKeys struct {
	id       string
	headsKey []byte
	rootKey  []byte
}

func newTreeKeys(id string) treeKeys {
	return treeKeys{
		id:       id,
		headsKey: joinStringsToBytes("t", id, "heads"),
		rootKey:  joinStringsToBytes("t", id, "rootId"),
	}
}

func (t treeKeys) HeadsKey() []byte {
	return t.headsKey
}

func (t treeKeys) RootIdKey() []byte {
	return t.rootKey
}

func (t treeKeys) RawChangeKey(id string) []byte {
	return joinStringsToBytes("t", t.id, id)
}

type spaceKeys struct {
	headerKey []byte
}

func newSpaceKeys(spaceId string) spaceKeys {
	return spaceKeys{headerKey: joinStringsToBytes("s", spaceId)}
}

var spaceIdKey = []byte("spaceId")

func (s spaceKeys) SpaceIdKey() []byte {
	return spaceIdKey
}

func (s spaceKeys) HeaderKey() []byte {
	return s.headerKey
}

func isRootIdKey(key string) bool {
	return strings.HasPrefix(key, "t/") && strings.HasSuffix(key, "rootId")
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

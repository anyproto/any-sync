package storage

import (
	"bytes"
)

type aclKeys struct {
	spaceId string
	rootKey []byte
	headKey []byte
}

func newACLKeys(spaceId string) aclKeys {
	return aclKeys{
		spaceId: spaceId,
		rootKey: joinStringsToBytes("space", spaceId, "a", "rootId"),
		headKey: joinStringsToBytes("space", spaceId, "a", "headId"),
	}
}

func (a aclKeys) HeadIdKey() []byte {
	return a.headKey
}

func (a aclKeys) RootIdKey() []byte {
	return a.rootKey
}

func (a aclKeys) RawRecordKey(id string) []byte {
	return joinStringsToBytes("space", a.spaceId, "a", id)
}

type treeKeys struct {
	id       string
	spaceId  string
	headsKey []byte
	rootKey  []byte
}

func newTreeKeys(spaceId, id string) treeKeys {
	return treeKeys{
		id:       id,
		spaceId:  spaceId,
		headsKey: joinStringsToBytes("space", spaceId, "t", id, "heads"),
		rootKey:  joinStringsToBytes("space", spaceId, "t", id),
	}
}

func (t treeKeys) HeadsKey() []byte {
	return t.headsKey
}

func (t treeKeys) RootIdKey() []byte {
	return t.rootKey
}

func (t treeKeys) RawChangeKey(id string) []byte {
	return joinStringsToBytes("space", t.spaceId, "t", t.id, id)
}

type spaceKeys struct {
	headerKey []byte
}

func newSpaceKeys(spaceId string) spaceKeys {
	return spaceKeys{headerKey: joinStringsToBytes("space", spaceId)}
}

func (s spaceKeys) HeaderKey() []byte {
	return s.headerKey
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

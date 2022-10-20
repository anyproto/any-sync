package storage

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
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
	return storage.JoinStringsToBytes("a", id)
}

type treeKeys struct {
	id       string
	headsKey []byte
}

func newTreeKeys(id string) treeKeys {
	return treeKeys{
		id:       id,
		headsKey: storage.JoinStringsToBytes("t", id, "heads"),
	}
}

func (t treeKeys) HeadsKey() []byte {
	return t.headsKey
}

func (t treeKeys) RawChangeKey(id string) []byte {
	return storage.JoinStringsToBytes("t", t.id, id)
}

type spaceKeys struct {
	headerKey []byte
}

func newSpaceKeys(spaceId string) spaceKeys {
	return spaceKeys{headerKey: storage.JoinStringsToBytes("s", spaceId)}
}

var spaceIdKey = []byte("spaceId")

func (s spaceKeys) SpaceIdKey() []byte {
	return spaceIdKey
}

func (s spaceKeys) HeaderKey() []byte {
	return s.headerKey
}

func isRootIdKey(key string) bool {
	return strings.HasPrefix(key, "t/") && strings.HasSuffix(key, "heads")
}

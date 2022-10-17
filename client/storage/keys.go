package storage

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
)

type aclKeys struct {
	spaceId string
	rootKey []byte
	headKey []byte
}

func newACLKeys(spaceId string) aclKeys {
	return aclKeys{
		spaceId: spaceId,
		rootKey: storage.JoinStringsToBytes("space", spaceId, "a", "rootId"),
		headKey: storage.JoinStringsToBytes("space", spaceId, "a", "headId"),
	}
}

func (a aclKeys) HeadIdKey() []byte {
	return a.headKey
}

func (a aclKeys) RootIdKey() []byte {
	return a.rootKey
}

func (a aclKeys) RawRecordKey(id string) []byte {
	return storage.JoinStringsToBytes("space", a.spaceId, "a", id)
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
		headsKey: storage.JoinStringsToBytes("space", spaceId, "t", id, "heads"),
		rootKey:  storage.JoinStringsToBytes("space", spaceId, "t", "rootId", id),
	}
}

func (t treeKeys) HeadsKey() []byte {
	return t.headsKey
}

func (t treeKeys) RootIdKey() []byte {
	return t.rootKey
}

func (t treeKeys) RawChangeKey(id string) []byte {
	return storage.JoinStringsToBytes("space", t.spaceId, "t", t.id, id)
}

type spaceKeys struct {
	headerKey     []byte
	treePrefixKey []byte
}

func newSpaceKeys(spaceId string) spaceKeys {
	return spaceKeys{
		headerKey:     storage.JoinStringsToBytes("space", spaceId),
		treePrefixKey: storage.JoinStringsToBytes("space", spaceId, "t", "rootId"),
	}
}

func (s spaceKeys) HeaderKey() []byte {
	return s.headerKey
}

func (s spaceKeys) TreeRootPrefix() []byte {
	return s.treePrefixKey
}

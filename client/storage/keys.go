package storage

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/treestorage"
)

type aclKeys struct {
	spaceId string
	rootKey []byte
	headKey []byte
}

func newACLKeys(spaceId string) aclKeys {
	return aclKeys{
		spaceId: spaceId,
		rootKey: treestorage.JoinStringsToBytes("space", spaceId, "a", "rootId"),
		headKey: treestorage.JoinStringsToBytes("space", spaceId, "a", "headId"),
	}
}

func (a aclKeys) HeadIdKey() []byte {
	return a.headKey
}

func (a aclKeys) RootIdKey() []byte {
	return a.rootKey
}

func (a aclKeys) RawRecordKey(id string) []byte {
	return treestorage.JoinStringsToBytes("space", a.spaceId, "a", id)
}

type treeKeys struct {
	id              string
	spaceId         string
	headsKey        []byte
	rootKey         []byte
	rawChangePrefix []byte
}

func newTreeKeys(spaceId, id string) treeKeys {
	return treeKeys{
		id:              id,
		spaceId:         spaceId,
		headsKey:        treestorage.JoinStringsToBytes("space", spaceId, "t", id, "heads"),
		rootKey:         treestorage.JoinStringsToBytes("space", spaceId, "t", "rootId", id),
		rawChangePrefix: treestorage.JoinStringsToBytes("space", spaceId, "t", id),
	}
}

func (t treeKeys) HeadsKey() []byte {
	return t.headsKey
}

func (t treeKeys) RootIdKey() []byte {
	return t.rootKey
}

func (t treeKeys) RawChangeKey(id string) []byte {
	return treestorage.JoinStringsToBytes("space", t.spaceId, "t", t.id, id)
}

func (t treeKeys) RawChangePrefix() []byte {
	return t.rawChangePrefix
}

type spaceKeys struct {
	spaceId            string
	headerKey          []byte
	treePrefixKey      []byte
	spaceSettingsIdKey []byte
}

func newSpaceKeys(spaceId string) spaceKeys {
	return spaceKeys{
		spaceId:            spaceId,
		headerKey:          treestorage.JoinStringsToBytes("space", "header", spaceId),
		treePrefixKey:      treestorage.JoinStringsToBytes("space", spaceId, "t", "rootId"),
		spaceSettingsIdKey: treestorage.JoinStringsToBytes("space", spaceId, "spaceSettingsId"),
	}
}

func (s spaceKeys) HeaderKey() []byte {
	return s.headerKey
}

func (s spaceKeys) TreeRootPrefix() []byte {
	return s.treePrefixKey
}

func (s spaceKeys) SpaceSettingsId() []byte {
	return s.spaceSettingsIdKey
}

func (s spaceKeys) TreeDeletedKey(id string) []byte {
	return treestorage.JoinStringsToBytes("space", s.spaceId, "deleted", id)
}

type storageServiceKeys struct {
	spacePrefix []byte
}

func newStorageServiceKeys() storageServiceKeys {
	return storageServiceKeys{spacePrefix: []byte("space/header")}
}

func (s storageServiceKeys) SpacePrefix() []byte {
	return s.spacePrefix
}

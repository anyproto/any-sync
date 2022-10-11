package storage

import (
	"fmt"
	"strings"
)

type treeKeys struct {
	id string
}

func (t treeKeys) HeadsKey() []byte {
	return []byte(fmt.Sprintf("t/%s/heads", t.id))
}

func (t treeKeys) RootKey() []byte {
	return []byte(fmt.Sprintf("t/%s", t.id))
}

func (t treeKeys) RawChangeKey(id string) []byte {
	return []byte(fmt.Sprintf("t/%s/%s", t.id, id))
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
	return strings.HasPrefix(key, "t/") && len(strings.Split(key, "/")) == 2
}

package storage

import (
	"fmt"
	"strings"
)

type treeKeys struct {
	id string
}

func (t treeKeys) HeadsKey() string {
	return fmt.Sprintf("%s/heads", t.id)
}

func (t treeKeys) RootKey() string {
	return fmt.Sprintf("t/%s", t.id)
}

func (t treeKeys) RawChangeKey(id string) string {
	return fmt.Sprintf("%s/%s", t.id, id)
}

type spaceKeys struct {
}

func (s spaceKeys) HeaderKey() string {
	return "header"
}

func (s spaceKeys) ACLKey() string {
	return "acl"
}

func isTreeKey(path string) bool {
	return strings.HasPrefix(path, "t/")
}

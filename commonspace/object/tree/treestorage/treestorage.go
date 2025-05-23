package treestorage

import (
	"bytes"
	"errors"
	"strings"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
)

var (
	ErrUnknownTreeId = errors.New("tree does not exist")
	ErrTreeExists    = errors.New("tree already exists")
	ErrUnknownChange = errors.New("change doesn't exist")
)

type TreeStorageCreatePayload struct {
	RootRawChange *treechangeproto.RawTreeChangeWithId
	Changes       []*treechangeproto.RawTreeChangeWithId
	Heads         []string
}

func ParseHeads(headsPayload []byte) []string {
	return strings.Split(string(headsPayload), "/")
}

func CreateHeadsPayload(heads []string) []byte {
	return JoinStringsToBytes(heads...)
}

func JoinStringsToBytes(strs ...string) []byte {
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

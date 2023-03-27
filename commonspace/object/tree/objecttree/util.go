package objecttree

import (
	"fmt"
	"github.com/anytypeio/any-sync/util/crypto"
)

func commonSnapshotForTwoPaths(ourPath []string, theirPath []string) (string, error) {
	var i int
	var j int
OuterLoop:
	// find starting point from the right
	for i = len(ourPath) - 1; i >= 0; i-- {
		for j = len(theirPath) - 1; j >= 0; j-- {
			// most likely there would be only one comparison, because mostly the snapshot path will start from the root for nodes
			if ourPath[i] == theirPath[j] {
				break OuterLoop
			}
		}
	}
	if i < 0 || j < 0 {
		return "", ErrNoCommonSnapshot
	}
	// find last common element of the sequence moving from right to left
	for i >= 0 && j >= 0 {
		if ourPath[i] == theirPath[j] {
			i--
			j--
		} else {
			break
		}
	}
	return ourPath[i+1], nil
}

func deriveTreeKey(key crypto.SymKey, cid string) (crypto.SymKey, error) {
	raw, err := key.Raw()
	if err != nil {
		return nil, err
	}
	return crypto.DeriveSymmetricKey(raw, fmt.Sprintf(crypto.AnysyncTreePath, cid))
}

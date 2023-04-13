package crypto

import (
	"github.com/anytypeio/go-slip21"
)

const (
	AnysyncSpacePath = "m/SLIP-0021/anysync/space"
	AnysyncTreePath  = "m/SLIP-0021/anysync/tree/%s"
)

// DeriveSymmetricKey derives a symmetric key from seed and path using slip-21
func DeriveSymmetricKey(seed []byte, path string) (SymKey, error) {
	master, err := slip21.DeriveForPath(path, seed)
	if err != nil {
		return nil, err
	}
	key, err := UnmarshallAESKey(master.SymmetricKey())
	if err != nil {
		return nil, err
	}
	return key, nil
}

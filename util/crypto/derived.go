package crypto

import (
	"github.com/anyproto/go-slip21"
)

const (
	AnysyncSpacePath             = "m/SLIP-0021/anysync/space"
	AnysyncTreePath              = "m/SLIP-0021/anysync/tree/%s"
	AnysyncKeyValuePath          = "m/SLIP-0021/anysync/keyvalue/%s"
	AnysyncOneToOneSpacePath     = "m/SLIP-0021/anysync/onetoone"
	AnysyncReadOneToOneSpacePath = "m/SLIP-0021/anysync/onetooneread"
	AnysyncMetadataOneToOnePath  = "m/SLIP-0021/anysync/onetoonemeta"
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

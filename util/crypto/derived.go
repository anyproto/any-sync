package crypto

import "github.com/anytypeio/go-slip21"

const AnytypeAccountPath = "m/SLIP-0021/anytype/account"

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

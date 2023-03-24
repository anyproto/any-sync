package crypto

import "github.com/anytypeio/go-slip21"

const anytypeAccountPath = "m/SLIP-0021/anytype/account"

func DeriveAccountSymmetric(seed []byte) (SymKey, error) {
	master, err := slip21.DeriveForPath(anytypeAccountPath, seed)
	if err != nil {
		return nil, err
	}
	key, err := UnmarshallAESKey(master.SymmetricKey())
	if err != nil {
		return nil, err
	}
	return key, nil
}

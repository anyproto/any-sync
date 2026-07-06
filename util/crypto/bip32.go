package crypto

import (
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

// Minimal BIP32 private-key derivation, replacing github.com/btcsuite/btcutil/hdkeychain.
//
// It reproduces hdkeychain's DeriveNonStandard exactly, including the
// non-standard behaviour of btcsuite issue #172: when an intermediate private
// key is shorter than 32 bytes, hardened derivation uses it left-aligned
// without zero-padding. Existing Ethereum identities were derived this way,
// so the quirk must be preserved bit-for-bit.

const hardenedKeyStart = 0x80000000

var (
	errUnusableSeed   = errors.New("unusable seed")
	errInvalidSeedLen = errors.New("seed length must be between 128 and 512 bits")
	errInvalidChild   = errors.New("the extended key at this index is invalid")

	bip32MasterHMACKey = []byte("Bitcoin seed")
	secp256k1N         = secp256k1.S256().N
)

// hdKey is an extended private key: the key bytes are big-endian and, matching
// hdkeychain, may be shorter than 32 bytes (big.Int.Bytes drops leading zeros).
type hdKey struct {
	key       []byte
	chainCode []byte
}

// newMasterHDKey derives the BIP32 master key from a seed,
// as hdkeychain.NewMaster does.
func newMasterHDKey(seed []byte) (*hdKey, error) {
	if len(seed) < 16 || len(seed) > 64 {
		return nil, errInvalidSeedLen
	}

	hmac512 := hmac.New(sha512.New, bip32MasterHMACKey)
	hmac512.Write(seed)
	lr := hmac512.Sum(nil)

	secretKey := lr[:len(lr)/2]
	chainCode := lr[len(lr)/2:]

	secretKeyNum := new(big.Int).SetBytes(secretKey)
	if secretKeyNum.Cmp(secp256k1N) >= 0 || secretKeyNum.Sign() == 0 {
		return nil, errUnusableSeed
	}

	return &hdKey{key: secretKey, chainCode: chainCode}, nil
}

// deriveNonStandard derives a child private key at the given index,
// as hdkeychain.(*ExtendedKey).DeriveNonStandard does.
func (k *hdKey) deriveNonStandard(i uint32) (*hdKey, error) {
	const keyLen = 33
	data := make([]byte, keyLen+4)
	if i >= hardenedKeyStart {
		copy(data[1:], k.key)
	} else {
		copy(data, k.pubKeyBytes())
	}
	binary.BigEndian.PutUint32(data[keyLen:], i)

	hmac512 := hmac.New(sha512.New, k.chainCode)
	hmac512.Write(data)
	ilr := hmac512.Sum(nil)

	il := ilr[:len(ilr)/2]
	childChainCode := ilr[len(ilr)/2:]

	ilNum := new(big.Int).SetBytes(il)
	if ilNum.Cmp(secp256k1N) >= 0 || ilNum.Sign() == 0 {
		return nil, errInvalidChild
	}

	keyNum := new(big.Int).SetBytes(k.key)
	ilNum.Add(ilNum, keyNum)
	ilNum.Mod(ilNum, secp256k1N)

	return &hdKey{key: ilNum.Bytes(), chainCode: childChainCode}, nil
}

func (k *hdKey) pubKeyBytes() []byte {
	return secp256k1.PrivKeyFromBytes(k.key).PubKey().SerializeCompressed()
}

// ecdsaPrivateKey converts the key to *ecdsa.PrivateKey,
// as hdkeychain's ECPrivKey().ToECDSA() does.
func (k *hdKey) ecdsaPrivateKey() *ecdsa.PrivateKey {
	return secp256k1.PrivKeyFromBytes(k.key).ToECDSA()
}

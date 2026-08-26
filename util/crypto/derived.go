package crypto

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha512"
	"errors"
	"strings"

	"github.com/anyproto/go-slip10"
	"github.com/anyproto/go-slip21"
)

const (
	AnysyncSpacePath             = "m/SLIP-0021/anysync/space"
	AnysyncTreePath              = "m/SLIP-0021/anysync/tree/%s"
	AnysyncKeyValuePath          = "m/SLIP-0021/anysync/keyvalue/%s"
	AnysyncOneToOneSpacePath     = "m/SLIP-0021/anysync/onetoone/0"
	AnysyncReadOneToOneSpacePath = "m/SLIP-0021/anysync/onetooneread"
	AnysyncMetadataOneToOnePath  = "m/SLIP-0021/anysync/onetoonemeta"

	// AnysyncDiscoveryKeyPath is the hardened slip-10 path of the key that
	// addresses and signs the account's device-discovery record. Its root
	// seed is the identity key's 32-byte ed25519 seed (not PrivKey.Raw()),
	// a fresh slip-10 root unrelated to the mnemonic tree. Frozen: devices
	// and restores must agree.
	AnysyncDiscoveryKeyPath = "m/99999'/2'"
	// AnysyncDiscoveryEncPath is the slip-21 path of the symmetric key that
	// encrypts that record.
	AnysyncDiscoveryEncPath = "m/SLIP-0021/anysync/discovery"
)

// DeriveDiscoveryKeys returns the account's device-discovery keys: an ed25519
// key that addresses and signs the discovery record and a symmetric key that
// encrypts its payload. Both derive from the identity key seed, so every
// device of the account and a fresh restore compute the same keys, while the
// identity key never signs the record and its public id stays unlinkable to
// it.
func DeriveDiscoveryKeys(identity PrivKey) (signKey PrivKey, encKey SymKey, err error) {
	if _, ok := identity.(*Ed25519PrivKey); !ok {
		return nil, nil, ErrIncorrectKeyType
	}
	raw, err := identity.Raw()
	if err != nil {
		return nil, nil, err
	}
	if len(raw) != ed25519.PrivateKeySize {
		return nil, nil, errors.New("identity key has an unexpected size")
	}
	seed := raw[:ed25519.SeedSize]
	node, err := slip10.DeriveForPath(AnysyncDiscoveryKeyPath, seed)
	if err != nil {
		return nil, nil, err
	}
	_, priv := node.Keypair()
	signKey = NewEd25519PrivKey(priv)
	if encKey, err = DeriveSymmetricKey(seed, AnysyncDiscoveryEncPath); err != nil {
		return nil, nil, err
	}
	return signKey, encKey, nil
}

const slip21SeedModifier = "Symmetric key seed"

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

// KeyDeriver pre-computes SLIP-21 path labels to allow efficient repeated
// derivation of symmetric keys from different seeds along the same path.
// This avoids per-call overhead of fmt.Sprintf, strings.Split, regex
// validation, and intermediate Node allocations in go-slip21.
type KeyDeriver struct {
	// labels stores pre-computed derivation labels with 0x00 prefix as required by SLIP-21
	labels [][]byte
	// buf is reused for HMAC-SHA512 output (64 bytes)
	buf []byte
}

// NewKeyDeriver creates a KeyDeriver for the given SLIP-21 path.
// The path should be fully formed, e.g. "m/SLIP-0021/anysync/tree/someId".
func NewKeyDeriver(path string) *KeyDeriver {
	path, _ = strings.CutPrefix(path, "m/")
	parts := strings.Split(path, "/")
	labels := make([][]byte, len(parts))
	for i, p := range parts {
		label := make([]byte, 1+len(p))
		label[0] = 0x00
		copy(label[1:], p)
		labels[i] = label
	}
	return &KeyDeriver{
		labels: labels,
		buf:    make([]byte, 0, 64),
	}
}

// DeriveKey derives a symmetric key from the given seed using
// the pre-computed path labels. This is equivalent to DeriveSymmetricKey
// but avoids repeated allocations when called in a loop.
func (d *KeyDeriver) DeriveKey(seed []byte) (SymKey, error) {
	// Master node derivation (equivalent to slip21.NewMasterNode)
	h := hmac.New(sha512.New, []byte(slip21SeedModifier))
	h.Write(seed)
	sum := h.Sum(d.buf[:0])

	// Derive through each label (equivalent to Node.Derive)
	for _, label := range d.labels {
		h = hmac.New(sha512.New, sum[:32]) // chainCode
		h.Write(label)
		sum = h.Sum(d.buf[:0])
	}

	// Copy the key bytes since sum references our reusable buffer
	key := make([]byte, 32)
	copy(key, sum[32:])
	return UnmarshallAESKey(key)
}

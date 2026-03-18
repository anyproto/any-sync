package crypto

import (
	"crypto/hmac"
	"crypto/sha512"
	"strings"

	"github.com/anyproto/go-slip21"
)

const (
	AnysyncSpacePath             = "m/SLIP-0021/anysync/space"
	AnysyncTreePath              = "m/SLIP-0021/anysync/tree/%s"
	AnysyncKeyValuePath          = "m/SLIP-0021/anysync/keyvalue/%s"
	AnysyncOneToOneSpacePath     = "m/SLIP-0021/anysync/onetoone/0"
	AnysyncReadOneToOneSpacePath = "m/SLIP-0021/anysync/onetooneread"
	AnysyncMetadataOneToOnePath  = "m/SLIP-0021/anysync/onetoonemeta"
)

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

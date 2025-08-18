package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"github.com/anyproto/any-sync/util/crypto/cryptoproto"
	mbase "github.com/multiformats/go-multibase"
)

const (
	// NonceBytes is the length of GCM nonce.
	NonceBytes = 12

	// KeyBytes is the length of GCM key.
	KeyBytes = 32
)

type AESKey struct {
	raw []byte
}

func (k *AESKey) Equals(key Key) bool {
	aesKey, ok := key.(*AESKey)
	if !ok {
		return false
	}

	return subtle.ConstantTimeCompare(k.raw, aesKey.raw) == 1
}

func (k *AESKey) Raw() ([]byte, error) {
	return k.raw, nil
}

// NewRandomAES returns a random key.
func NewRandomAES() (*AESKey, error) {
	raw := make([]byte, KeyBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	return &AESKey{raw: raw}, nil
}

// NewAES returns AESKey if err is nil and panics otherwise.
func NewAES() *AESKey {
	k, err := NewRandomAES()
	if err != nil {
		panic(err)
	}
	return k
}

// UnmarshallAESKey returns a key by decoding bytes.
func UnmarshallAESKey(k []byte) (*AESKey, error) {
	if len(k) != KeyBytes {
		return nil, fmt.Errorf("invalid key")
	}
	return &AESKey{raw: k}, nil
}

// UnmarshallAESKeyProto returns a key by decoding bytes.
func UnmarshallAESKeyProto(k []byte) (*AESKey, error) {
	msg := &cryptoproto.Key{}
	err := msg.UnmarshalVT(k)
	if err != nil {
		return nil, err
	}
	if msg.Type != cryptoproto.KeyType_AES {
		return nil, ErrIncorrectKeyType
	}
	return UnmarshallAESKey(msg.Data)
}

// UnmarshallAESKeyString returns a key by decoding a base32-encoded string.
func UnmarshallAESKeyString(k string) (*AESKey, error) {
	_, b, err := mbase.Decode(k)
	if err != nil {
		return nil, err
	}
	return UnmarshallAESKey(b)
}

// Bytes returns raw key bytes.
func (k *AESKey) Bytes() []byte {
	return k.raw
}

// String returns the base32-encoded string representation of raw key bytes.
func (k *AESKey) String() string {
	str, err := mbase.Encode(mbase.Base32, k.raw)
	if err != nil {
		panic("should not error with hardcoded mbase: " + err.Error())
	}
	return str
}

// Encrypt performs AES-256 GCM encryption on plaintext.
func (k *AESKey) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(k.raw[:KeyBytes])
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, NonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)
	ciphertext = append(nonce[:], ciphertext...)
	return ciphertext, nil
}

// Decrypt uses key to perform AES-256 GCM decryption on ciphertext.
func (k *AESKey) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(k.raw[:KeyBytes])
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := ciphertext[:NonceBytes]
	plain, err := aesgcm.Open(nil, nonce, ciphertext[NonceBytes:], nil)
	if err != nil {
		return nil, err
	}
	return plain, nil
}

// Marshall marshalls the key into proto
func (k *AESKey) Marshall() ([]byte, error) {
	msg := &cryptoproto.Key{
		Type: cryptoproto.KeyType_AES,
		Data: k.raw,
	}
	return msg.MarshalVT()
}

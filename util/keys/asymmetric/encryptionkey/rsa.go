package encryptionkey

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/subtle"
	"crypto/x509"
	"errors"
	"github.com/anytypeio/any-sync/util/keys"
	"io"
)

var MinRsaKeyBits = 2048

var ErrKeyLengthTooSmall = errors.New("error key length too small")

type EncryptionRsaPrivKey struct {
	privKey rsa.PrivateKey
}

type EncryptionRsaPubKey struct {
	pubKey rsa.PublicKey
}

func (e *EncryptionRsaPubKey) Equals(key keys.Key) bool {
	other, ok := (key).(*EncryptionRsaPubKey)
	if !ok {
		return keyEquals(e, key)
	}

	return e.pubKey.N.Cmp(other.pubKey.N) == 0 && e.pubKey.E == other.pubKey.E
}

func (e *EncryptionRsaPubKey) Raw() ([]byte, error) {
	return x509.MarshalPKIXPublicKey(&e.pubKey)
}

func (e *EncryptionRsaPubKey) Encrypt(data []byte) ([]byte, error) {
	hash := sha512.New()
	return rsa.EncryptOAEP(hash, rand.Reader, &e.pubKey, data, nil)
}

func (e *EncryptionRsaPrivKey) Equals(key keys.Key) bool {
	other, ok := (key).(*EncryptionRsaPrivKey)
	if !ok {
		return keyEquals(e, key)
	}

	return e.privKey.N.Cmp(other.privKey.N) == 0 && e.privKey.E == other.privKey.E
}

func (e *EncryptionRsaPrivKey) Raw() ([]byte, error) {
	b := x509.MarshalPKCS1PrivateKey(&e.privKey)
	return b, nil
}

func (e *EncryptionRsaPrivKey) Decrypt(bytes []byte) ([]byte, error) {
	hash := sha512.New()
	return rsa.DecryptOAEP(hash, rand.Reader, &e.privKey, bytes, nil)
}

func (e *EncryptionRsaPrivKey) GetPublic() PubKey {
	return &EncryptionRsaPubKey{pubKey: e.privKey.PublicKey}
}

func GenerateRandomRSAKeyPair(bits int) (PrivKey, PubKey, error) {
	return GenerateRSAKeyPair(bits, rand.Reader)
}

func GenerateRSAKeyPair(bits int, src io.Reader) (PrivKey, PubKey, error) {
	if bits < MinRsaKeyBits {
		return nil, nil, ErrKeyLengthTooSmall
	}
	priv, err := rsa.GenerateKey(src, bits)
	if err != nil {
		return nil, nil, err
	}
	pk := priv.PublicKey
	return &EncryptionRsaPrivKey{privKey: *priv}, &EncryptionRsaPubKey{pubKey: pk}, nil
}

func NewEncryptionRsaPrivKeyFromBytes(bytes []byte) (PrivKey, error) {
	sk, err := x509.ParsePKCS1PrivateKey(bytes)
	if err != nil {
		return nil, err
	}
	if sk.N.BitLen() < MinRsaKeyBits {
		return nil, ErrKeyLengthTooSmall
	}
	return &EncryptionRsaPrivKey{privKey: *sk}, nil
}

func NewEncryptionRsaPubKeyFromBytes(bytes []byte) (PubKey, error) {
	pub, err := x509.ParsePKIXPublicKey(bytes)
	if err != nil {
		return nil, err
	}
	pk, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not actually an rsa public key")
	}
	if pk.N.BitLen() < MinRsaKeyBits {
		return nil, ErrKeyLengthTooSmall
	}

	return &EncryptionRsaPubKey{pubKey: *pk}, nil
}

func keyEquals(k1, k2 keys.Key) bool {
	a, err := k1.Raw()
	if err != nil {
		return false
	}
	b, err := k2.Raw()
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(a, b) == 1
}

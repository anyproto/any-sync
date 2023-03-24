package crypto

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
)

// Ed25519PrivKey is an ed25519 private key.
type Ed25519PrivKey struct {
	privKey   ed25519.PrivateKey
	privCurve *[32]byte
	pubCurve  *[32]byte
}

// Ed25519PubKey is an ed25519 public key.
type Ed25519PubKey struct {
	pubKey   ed25519.PublicKey
	pubCurve *[32]byte
}

func NewEd25519PrivKey(privKey ed25519.PrivateKey) PrivKey {
	pK := &Ed25519PrivKey{privKey: privKey}
	pubKey := pK.pubKeyBytes()
	privCurve := Ed25519PrivateKeyToCurve25519(privKey)
	pubCurve := Ed25519PublicKeyToCurve25519(pubKey)
	pK.privCurve = (*[32]byte)(privCurve)
	pK.pubCurve = (*[32]byte)(pubCurve)
	return pK
}

func NewEd25519PubKey(pubKey ed25519.PublicKey) PubKey {
	pK := &Ed25519PubKey{pubKey: pubKey}
	pubCurve := Ed25519PublicKeyToCurve25519(pubKey)
	pK.pubCurve = (*[32]byte)(pubCurve)
	return pK
}

func NewSigningEd25519PubKeyFromBytes(bytes []byte) (PubKey, error) {
	return UnmarshalEd25519PublicKey(bytes)
}

func NewSigningEd25519PrivKeyFromBytes(bytes []byte) (PrivKey, error) {
	return UnmarshalEd25519PrivateKey(bytes)
}

func GenerateRandomEd25519KeyPair() (PrivKey, PubKey, error) {
	return GenerateEd25519Key(rand.Reader)
}

// GenerateEd25519Key generates a new ed25519 private and public key pair.
func GenerateEd25519Key(src io.Reader) (PrivKey, PubKey, error) {
	pub, priv, err := ed25519.GenerateKey(src)
	if err != nil {
		return nil, nil, err
	}

	return NewEd25519PrivKey(priv),
		NewEd25519PubKey(pub),
		nil
}

// Raw private key bytes.
func (k *Ed25519PrivKey) Raw() ([]byte, error) {
	buf := make([]byte, len(k.privKey))
	copy(buf, k.privKey)

	return buf, nil
}

func (k *Ed25519PrivKey) pubKeyBytes() []byte {
	return k.privKey[ed25519.PrivateKeySize-ed25519.PublicKeySize:]
}

// Equals compares two ed25519 private keys.
func (k *Ed25519PrivKey) Equals(o Key) bool {
	edk, ok := o.(*Ed25519PrivKey)
	if !ok {
		return KeyEquals(k, o)
	}

	return subtle.ConstantTimeCompare(k.privKey, edk.privKey) == 1
}

// GetPublic returns an ed25519 public key from a private key.
func (k *Ed25519PrivKey) GetPublic() PubKey {
	return &Ed25519PubKey{
		pubKey:   k.pubKeyBytes(),
		pubCurve: k.pubCurve,
	}
}

// Sign returns a signature from an input message.
func (k *Ed25519PrivKey) Sign(msg []byte) ([]byte, error) {
	return ed25519.Sign(k.privKey, msg), nil
}

// Decrypt decrypts the message
func (k *Ed25519PrivKey) Decrypt(msg []byte) ([]byte, error) {
	return DecryptX25519(k.privCurve, k.pubCurve, msg)
}

// Raw public key bytes.
func (k *Ed25519PubKey) Raw() ([]byte, error) {
	return k.pubKey, nil
}

// Encrypt message
func (k *Ed25519PubKey) Encrypt(msg []byte) (data []byte, err error) {
	data = EncryptX25519(k.pubCurve, msg)
	return
}

// Equals compares two ed25519 public keys.
func (k *Ed25519PubKey) Equals(o Key) bool {
	edk, ok := o.(*Ed25519PubKey)
	if !ok {
		return KeyEquals(k, o)
	}

	return bytes.Equal(k.pubKey, edk.pubKey)
}

// Verify checks a signature agains the input data.
func (k *Ed25519PubKey) Verify(data []byte, sig []byte) (bool, error) {
	return ed25519.Verify(k.pubKey, data, sig), nil
}

// UnmarshalEd25519PublicKey returns a public key from input bytes.
func UnmarshalEd25519PublicKey(data []byte) (PubKey, error) {
	if len(data) != 32 {
		return nil, errors.New("expect ed25519 public key data size to be 32")
	}

	return NewEd25519PubKey(data), nil
}

// UnmarshalEd25519PrivateKey returns a private key from input bytes.
func UnmarshalEd25519PrivateKey(data []byte) (PrivKey, error) {
	switch len(data) {
	case ed25519.PrivateKeySize + ed25519.PublicKeySize:
		// Remove the redundant public key. See issue #36.
		redundantPk := data[ed25519.PrivateKeySize:]
		pk := data[ed25519.PrivateKeySize-ed25519.PublicKeySize : ed25519.PrivateKeySize]
		if subtle.ConstantTimeCompare(pk, redundantPk) == 0 {
			return nil, errors.New("expected redundant ed25519 public key to be redundant")
		}

		// No point in storing the extra data.
		newKey := make([]byte, ed25519.PrivateKeySize)
		copy(newKey, data[:ed25519.PrivateKeySize])
		data = newKey
	case ed25519.PrivateKeySize:
	default:
		return nil, fmt.Errorf(
			"expected ed25519 data size to be %d or %d, got %d",
			ed25519.PrivateKeySize,
			ed25519.PrivateKeySize+ed25519.PublicKeySize,
			len(data),
		)
	}

	return NewEd25519PrivKey(data), nil
}

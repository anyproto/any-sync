package crypto

import (
	"crypto/subtle"
	"errors"
	"github.com/libp2p/go-libp2p/core/crypto"
)

var ErrIncorrectKeyType = errors.New("incorrect key type")

// Key is an abstract interface for all types of keys
type Key interface {
	// Equals returns if the keys are equal
	Equals(Key) bool

	// Raw returns raw key
	Raw() ([]byte, error)
}

// PrivKey is an interface for keys that should be used for signing and decryption
type PrivKey interface {
	Key

	// Decrypt decrypts the message and returns the result
	Decrypt(message []byte) ([]byte, error)
	// Sign signs the raw bytes and returns the signature
	Sign([]byte) ([]byte, error)
	// GetPublic returns the associated public key
	GetPublic() PubKey
	// Marshall wraps key in proto encoding and marshalls it
	Marshall() ([]byte, error)
	// LibP2P returns libp2p model
	LibP2P() (crypto.PrivKey, error)
}

// PubKey is the public key used to verify the signatures and decrypt messages
type PubKey interface {
	Key

	// Encrypt encrypts the message and returns the result
	Encrypt(message []byte) ([]byte, error)
	// Verify verifies the signed message and the signature
	Verify(data []byte, sig []byte) (bool, error)
	// Marshall wraps key in proto encoding and marshalls it
	Marshall() ([]byte, error)
	// Storage returns underlying key storage
	Storage() []byte
	// Account returns string representation for anytype account
	Account() string
	// Network returns string representation for anytype network
	Network() string
	// PeerId returns string representation for peer id
	PeerId() string
	// LibP2P returns libp2p model
	LibP2P() (crypto.PubKey, error)
}

type SymKey interface {
	Key

	// Decrypt decrypts the message and returns the result
	Decrypt(message []byte) ([]byte, error)
	// Encrypt encrypts the message and returns the result
	Encrypt(message []byte) ([]byte, error)
	// Marshall wraps key in proto encoding and marshalls it
	Marshall() ([]byte, error)
}

func KeyEquals(k1, k2 Key) bool {
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

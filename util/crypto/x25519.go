package crypto

import (
	"crypto/rand"
	"errors"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/nacl/box"
)

var ErrX25519DecryptionFailed = errors.New("failed decryption with x25519 key")

// EncryptX25519 takes a x25519 public key and encrypts the message
func EncryptX25519(pubKey *[32]byte, msg []byte) []byte {
	// see discussion here https://github.com/golang/go/issues/29128
	var nonce [24]byte
	epk, esk, _ := box.GenerateKey(rand.Reader)
	// nonce logic is taken from libsodium https://github.com/jedisct1/libsodium/blob/master/src/libsodium/crypto_box/crypto_box_seal.c
	nonceWriter, _ := blake2b.New(24, nil)
	nonceSlice := nonceWriter.Sum(append(epk[:], pubKey[:]...))
	copy(nonce[:], nonceSlice)

	return box.Seal(epk[:], msg, &nonce, pubKey, esk)
}

// DecryptX25519 takes a x25519 private and public key and decrypts the message
func DecryptX25519(privKey, pubKey *[32]byte, encrypted []byte) ([]byte, error) {
	var epk [32]byte
	var nonce [24]byte
	copy(epk[:], encrypted[:32])

	nonceWriter, _ := blake2b.New(24, nil)
	nonceSlice := nonceWriter.Sum(append(epk[:], pubKey[:]...))
	copy(nonce[:], nonceSlice)

	decrypted, ok := box.Open(nil, encrypted[32:], &nonce, &epk, privKey)
	if !ok {
		return nil, ErrX25519DecryptionFailed
	}
	return decrypted, nil
}

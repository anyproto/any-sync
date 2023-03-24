package signingkey

import (
	"crypto/ed25519"
	"crypto/rand"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/signingkey/edwards25519"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/nacl/box"
)

type PrivKey interface {
	//crypto.Key

	Sign([]byte) ([]byte, error)
	GetPublic() PubKey
}

type PubKey interface {
	//crypto.Key

	Verify(data []byte, sig []byte) (bool, error)
}

func EncryptWithEd25519(pk ed25519.PublicKey, msg []byte) []byte {
	conv := edwards25519.Ed25519PublicKeyToCurve25519(pk)
	return Encrypt((*[32]byte)(conv), msg)
}

func DecryptWithEd25519(pub ed25519.PublicKey, priv ed25519.PrivateKey, msg []byte) []byte {
	cPub := edwards25519.Ed25519PublicKeyToCurve25519(pub)
	cPriv := edwards25519.Ed25519PrivateKeyToCurve25519(priv)
	return Decrypt((*[32]byte)(cPriv), (*[32]byte)(cPub), msg)
}

func Encrypt(pubKey *[32]byte, msg []byte) []byte {
	var nonce [24]byte
	epk, esk, _ := box.GenerateKey(rand.Reader)
	nonceWriter, _ := blake2b.New(24, nil)
	nonceSlice := nonceWriter.Sum(append(epk[:], pubKey[:]...))
	copy(nonce[:], nonceSlice)

	return box.Seal(epk[:], msg, &nonce, pubKey, esk)
}

func Decrypt(privKey, pubKey *[32]byte, encrypted []byte) []byte {
	var epk [32]byte
	var nonce [24]byte
	copy(epk[:], encrypted[:32])

	nonceWriter, _ := blake2b.New(24, nil)
	nonceSlice := nonceWriter.Sum(append(epk[:], pubKey[:]...))
	copy(nonce[:], nonceSlice)

	decrypted, ok := box.Open(nil, encrypted[32:], &nonce, &epk, privKey)
	if !ok {
		panic("Decryption error.")
	}
	return decrypted
}

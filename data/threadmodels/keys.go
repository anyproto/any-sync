package threadmodels

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/subtle"
	"crypto/x509"
	"errors"
	"io"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/strkey"
	"github.com/libp2p/go-libp2p-core/crypto"
	crypto_pb "github.com/libp2p/go-libp2p-core/crypto/pb"
)

type SigningPubKey crypto.PubKey
type SigningPrivKey crypto.PrivKey

var MinRsaKeyBits = 2048

var ErrKeyLengthTooSmall = errors.New("error key length too small")

type Key interface {
	Equals(Key) bool

	Raw() ([]byte, error)
}

type EncryptionPrivKey interface {
	Key

	Decrypt([]byte) ([]byte, error)
	GetPublic() EncryptionPubKey
}

type EncryptionPubKey interface {
	Key

	Encrypt(data []byte) ([]byte, error)
}

type EncryptionRsaPrivKey struct {
	privKey rsa.PrivateKey
}

type EncryptionRsaPubKey struct {
	pubKey rsa.PublicKey
}

func (e *EncryptionRsaPubKey) Equals(key Key) bool {
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

func (e *EncryptionRsaPrivKey) Equals(key Key) bool {
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

func (e *EncryptionRsaPrivKey) GetPublic() EncryptionPubKey {
	return &EncryptionRsaPubKey{pubKey: e.privKey.PublicKey}
}

func GenerateRandomRSAKeyPair(bits int) (EncryptionPrivKey, EncryptionPubKey, error) {
	return GenerateRSAKeyPair(bits, rand.Reader)
}

func GenerateRSAKeyPair(bits int, src io.Reader) (EncryptionPrivKey, EncryptionPubKey, error) {
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

func NewEncryptionRsaPrivKeyFromBytes(bytes []byte) (EncryptionPrivKey, error) {
	sk, err := x509.ParsePKCS1PrivateKey(bytes)
	if err != nil {
		return nil, err
	}
	if sk.N.BitLen() < MinRsaKeyBits {
		return nil, ErrKeyLengthTooSmall
	}
	return &EncryptionRsaPrivKey{privKey: *sk}, nil
}

func NewEncryptionRsaPubKeyFromBytes(bytes []byte) (EncryptionPubKey, error) {
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

func NewSigningEd25519PubKeyFromBytes(bytes []byte) (SigningPubKey, error) {
	return crypto.UnmarshalEd25519PublicKey(bytes)
}

func GenerateRandomEd25519KeyPair() (SigningPrivKey, SigningPubKey, error) {
	return crypto.GenerateEd25519Key(rand.Reader)
}

func keyEquals(k1, k2 Key) bool {
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

type Ed25519SigningPubKeyDecoder struct{}

func NewEd25519Decoder() *Ed25519SigningPubKeyDecoder {
	return &Ed25519SigningPubKeyDecoder{}
}

func (e *Ed25519SigningPubKeyDecoder) DecodeFromBytes(bytes []byte) (SigningPubKey, error) {
	return NewSigningEd25519PubKeyFromBytes(bytes)
}

func (e *Ed25519SigningPubKeyDecoder) DecodeFromString(identity string) (SigningPubKey, error) {
	pubKeyRaw, err := strkey.Decode(0x5b, identity)
	if err != nil {
		return nil, err
	}

	return e.DecodeFromBytes(pubKeyRaw)
}

func (e *Ed25519SigningPubKeyDecoder) DecodeFromStringIntoBytes(identity string) ([]byte, error) {
	return strkey.Decode(0x5b, identity)
}

func (e *Ed25519SigningPubKeyDecoder) EncodeToString(pubkey crypto.PubKey) (string, error) {
	raw, err := pubkey.Raw()
	if err != nil {
		return "", err
	}
	return strkey.Encode(0x5b, raw)
}

type SigningPubKeyDecoder interface {
	DecodeFromBytes(bytes []byte) (SigningPubKey, error)
	DecodeFromString(identity string) (SigningPubKey, error)
	DecodeFromStringIntoBytes(identity string) ([]byte, error)
}

// Below keys are required for testing and mocking purposes

type EmptyRecorderEncryptionKey struct {
	recordedEncrypted [][]byte
	recordedDecrypted [][]byte
}

func (f *EmptyRecorderEncryptionKey) Equals(key Key) bool {
	return true
}

func (f *EmptyRecorderEncryptionKey) Raw() ([]byte, error) {
	panic("can't get bytes from this key")
}

func (f *EmptyRecorderEncryptionKey) GetPublic() EncryptionPubKey {
	panic("this key doesn't have a public key")
}

func (f *EmptyRecorderEncryptionKey) Encrypt(msg []byte) ([]byte, error) {
	f.recordedEncrypted = append(f.recordedEncrypted, msg)
	return msg, nil
}

func (f *EmptyRecorderEncryptionKey) Decrypt(msg []byte) ([]byte, error) {
	f.recordedDecrypted = append(f.recordedDecrypted, msg)
	return msg, nil
}

type SignatureVerificationPayload struct {
	message   []byte
	signature []byte
}

type EmptyRecorderVerificationKey struct {
	verifications []SignatureVerificationPayload
}

func (e *EmptyRecorderVerificationKey) Bytes() ([]byte, error) {
	panic("can't get bytes from this key")
}

func (e *EmptyRecorderVerificationKey) Equals(key crypto.Key) bool {
	return true
}

func (e *EmptyRecorderVerificationKey) Raw() ([]byte, error) {
	panic("can't get bytes from this key")
}

func (e *EmptyRecorderVerificationKey) Type() crypto_pb.KeyType {
	panic("can't get type from this key")
}

func (e *EmptyRecorderVerificationKey) Verify(data []byte, sig []byte) (bool, error) {
	e.verifications = append(e.verifications, SignatureVerificationPayload{
		message:   data,
		signature: sig,
	})
	return true, nil
}

func NewMockSigningPubKeyFromBytes(bytes []byte) (SigningPubKey, error) {
	return &EmptyRecorderVerificationKey{}, nil
}

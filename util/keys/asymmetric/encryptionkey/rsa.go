package encryptionkey

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/subtle"
	"crypto/x509"
	"errors"
	"github.com/anytypeio/any-sync/util/crypto"
	"github.com/cespare/xxhash"
	mrand "golang.org/x/exp/rand"
	"io"
	"math"
	"math/big"
)

var bigZero = big.NewInt(0)
var bigOne = big.NewInt(1)

var MinRsaKeyBits = 2048

var ErrKeyLengthTooSmall = errors.New("error key length too small")

type EncryptionRsaPrivKey struct {
	privKey rsa.PrivateKey
}

type EncryptionRsaPubKey struct {
	pubKey rsa.PublicKey
}

func (e *EncryptionRsaPubKey) Equals(key crypto.Key) bool {
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

func (e *EncryptionRsaPrivKey) Equals(key crypto.Key) bool {
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

func DeriveRSAKePair(bits int, seed []byte) (PrivKey, PubKey, error) {
	if bits < MinRsaKeyBits {
		return nil, nil, ErrKeyLengthTooSmall
	}
	seed64 := xxhash.Sum64(seed)
	priv, err := rsaGenerateMultiPrimeKey(mrand.New(mrand.NewSource(seed64)), 2, bits)
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

func keyEquals(k1, k2 crypto.Key) bool {
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

// generateMultiPrimeKey is a copied original rsa.GenerateMultiPrimeKey but without randutil.MaybeReadByte calls
func rsaGenerateMultiPrimeKey(random io.Reader, nprimes int, bits int) (*rsa.PrivateKey, error) {

	priv := new(rsa.PrivateKey)
	priv.E = 65537

	if nprimes < 2 {
		return nil, errors.New("crypto/rsa: GenerateMultiPrimeKey: nprimes must be >= 2")
	}

	if bits < 64 {
		primeLimit := float64(uint64(1) << uint(bits/nprimes))
		// pi approximates the number of primes less than primeLimit
		pi := primeLimit / (math.Log(primeLimit) - 1)
		// Generated primes start with 11 (in binary) so we can only
		// use a quarter of them.
		pi /= 4
		// Use a factor of two to ensure that key generation terminates
		// in a reasonable amount of time.
		pi /= 2
		if pi <= float64(nprimes) {
			return nil, errors.New("crypto/rsa: too few primes of given length to generate an RSA key")
		}
	}

	primes := make([]*big.Int, nprimes)

NextSetOfPrimes:
	for {
		todo := bits
		// crypto/rand should set the top two bits in each prime.
		// Thus each prime has the form
		//   p_i = 2^bitlen(p_i) × 0.11... (in base 2).
		// And the product is:
		//   P = 2^todo × α
		// where α is the product of nprimes numbers of the form 0.11...
		//
		// If α < 1/2 (which can happen for nprimes > 2), we need to
		// shift todo to compensate for lost bits: the mean value of 0.11...
		// is 7/8, so todo + shift - nprimes * log2(7/8) ~= bits - 1/2
		// will give good results.
		if nprimes >= 7 {
			todo += (nprimes - 2) / 5
		}
		for i := 0; i < nprimes; i++ {
			var err error
			primes[i], err = randPrime(random, todo/(nprimes-i))
			if err != nil {
				return nil, err
			}
			todo -= primes[i].BitLen()
		}

		// Make sure that primes is pairwise unequal.
		for i, prime := range primes {
			for j := 0; j < i; j++ {
				if prime.Cmp(primes[j]) == 0 {
					continue NextSetOfPrimes
				}
			}
		}

		n := new(big.Int).Set(bigOne)
		totient := new(big.Int).Set(bigOne)
		pminus1 := new(big.Int)
		for _, prime := range primes {
			n.Mul(n, prime)
			pminus1.Sub(prime, bigOne)
			totient.Mul(totient, pminus1)
		}
		if n.BitLen() != bits {
			// This should never happen for nprimes == 2 because
			// crypto/rand should set the top two bits in each prime.
			// For nprimes > 2 we hope it does not happen often.
			continue NextSetOfPrimes
		}

		priv.D = new(big.Int)
		e := big.NewInt(int64(priv.E))
		ok := priv.D.ModInverse(e, totient)

		if ok != nil {
			priv.Primes = primes
			priv.N = n
			break
		}
	}

	priv.Precompute()
	return priv, nil
}

func randPrime(rand io.Reader, bits int) (*big.Int, error) {
	if bits < 2 {
		return nil, errors.New("crypto/rand: prime size must be at least 2-bit")
	}

	b := uint(bits % 8)
	if b == 0 {
		b = 8
	}

	bytes := make([]byte, (bits+7)/8)
	p := new(big.Int)

	for {
		if _, err := io.ReadFull(rand, bytes); err != nil {
			return nil, err
		}

		// Clear bits in the first byte to make sure the candidate has a size <= bits.
		bytes[0] &= uint8(int(1<<b) - 1)
		// Don't let the value be too small, i.e, set the most significant two bits.
		// Setting the top two bits, rather than just the top bit,
		// means that when two of these values are multiplied together,
		// the result isn't ever one bit short.
		if b >= 2 {
			bytes[0] |= 3 << (b - 2)
		} else {
			// Here b==1, because b cannot be zero.
			bytes[0] |= 1
			if len(bytes) > 1 {
				bytes[1] |= 0x80
			}
		}
		// Make the value odd since an even number this large certainly isn't prime.
		bytes[len(bytes)-1] |= 1

		p.SetBytes(bytes)
		if p.ProbablyPrime(20) {
			return p, nil
		}
	}
}

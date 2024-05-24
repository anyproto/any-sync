package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/anyproto/go-slip10"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/sha3"
)

func Keccak256(data []byte) []byte {
	hash := sha3.NewLegacyKeccak256()
	hash.Write(data)
	return hash.Sum(nil)
}

func PublicKeyToAddress(pub *ecdsa.PublicKey) string {
	// Serialize the public key to a byte slice
	pubBytes := elliptic.Marshal(pub.Curve, pub.X, pub.Y)

	// Compute the Keccak256 hash of the public key (excluding the first byte)
	hash := Keccak256(pubBytes[1:])

	// Take the last 20 bytes of the hash as the address
	address := hash[12:]

	return fmt.Sprintf("0x%x", address)
}

func PrivateKeyToBytes(priv *ecdsa.PrivateKey) []byte {
	return priv.D.Bytes()
}

// Encode encodes b as a hex string with 0x prefix.
func Encode(b []byte) string {
	enc := make([]byte, len(b)*2+2)
	copy(enc, "0x")
	hex.Encode(enc[2:], b)
	return string(enc)
}

func TestMnemonic(t *testing.T) {
	phrase, err := NewMnemonicGenerator().WithWordCount(12)
	require.NoError(t, err)
	parts := strings.Split(string(phrase), " ")
	require.Equal(t, 12, len(parts))
	res, err := phrase.DeriveKeys(0)
	require.NoError(t, err)
	bytes := make([]byte, 64)
	_, err = rand.Read(bytes)
	require.NoError(t, err)

	// testing signing with keys
	for _, k := range []PrivKey{res.MasterKey, res.Identity, res.OldAccountKey} {
		sign, err := k.Sign(bytes)
		require.NoError(t, err)
		res, err := k.GetPublic().Verify(bytes, sign)
		require.NoError(t, err)
		require.True(t, res)
	}

	// testing derivation
	masterKey, err := genKey(res.MasterNode)
	require.NoError(t, err)
	require.True(t, res.MasterKey.Equals(masterKey))
	identityNode, err := res.MasterNode.Derive(slip10.FirstHardenedIndex)
	require.NoError(t, err)
	identityKey, err := genKey(identityNode)
	require.NoError(t, err)
	require.True(t, res.Identity.Equals(identityKey))
	oldAccountRes, err := phrase.deriveForPath(true, 0, anytypeAccountOldPrefix)
	require.NoError(t, err)
	require.True(t, res.OldAccountKey.Equals(oldAccountRes.MasterKey))

	// testing Ethereum derivation:
	var phrase2 Mnemonic = "tag volcano eight thank tide danger coast health above argue embrace heavy"
	res, err = phrase2.DeriveKeys(0)
	require.NoError(t, err)

	// get address by public key
	publicKey := res.EthereumIdentity.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	require.Equal(t, true, ok)

	// convert publicKeyECDSA to address
	ethAddress := PublicKeyToAddress(publicKeyECDSA)
	shouldBe := strings.ToLower("0xC49926C4124cEe1cbA0Ea94Ea31a6c12318df947")
	require.Equal(t, shouldBe, ethAddress)
}

func TestMnemonic_ethereumKeyFromMnemonic(t *testing.T) {
	var badPphrase Mnemonic = "tag volcano"
	_, err := badPphrase.ethereumKeyFromMnemonic(0, defaultEthereumDerivation)
	require.Error(t, err)

	// good
	var phrase Mnemonic = "tag volcano eight thank tide danger coast health above argue embrace heavy"

	pk, err := phrase.ethereumKeyFromMnemonic(0, defaultEthereumDerivation)
	require.NoError(t, err)

	// check address
	publicKey := pk.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	require.Equal(t, true, ok)

	// convert publicKeyECDSA to address
	ethAddress := PublicKeyToAddress(publicKeyECDSA)
	shouldBe := strings.ToLower("0xC49926C4124cEe1cbA0Ea94Ea31a6c12318df947")
	require.Equal(t, shouldBe, ethAddress)

	// what wallet.PrivateKeyHex(account) does
	bytes := PrivateKeyToBytes(pk)
	pkStr := Encode(bytes)[2:]
	require.Equal(t, "63e21d10fd50155dbba0e7d3f7431a400b84b4c2ac1ee38872f82448fe3ecfb9", pkStr)

	pk, err = phrase.ethereumKeyFromMnemonic(1, defaultEthereumDerivation)
	require.NoError(t, err)

	// check address
	publicKey = pk.Public()
	publicKeyECDSA, ok = publicKey.(*ecdsa.PublicKey)
	require.Equal(t, true, ok)

	// convert publicKeyECDSA to address
	ethAddress = PublicKeyToAddress(publicKeyECDSA)
	shouldBe = strings.ToLower("0x8230645aC28A4EdD1b0B53E7Cd8019744E9dD559")
	require.Equal(t, shouldBe, ethAddress)

	bytes = PrivateKeyToBytes(pk)
	pkStr = Encode(bytes)[2:]
	require.Equal(t, "b31048b0aa87649bdb9016c0ee28c788ddfc45e52cd71cc0da08c47cb4390ae7", pkStr)
}

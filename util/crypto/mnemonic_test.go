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

func TestDeriveMasterNode(t *testing.T) {
	phrase, err := NewMnemonicGenerator().WithWordCount(12)
	require.NoError(t, err)

	// Test deriving master node for index 0
	masterNode0, err := phrase.DeriveMasterNode(0)
	require.NoError(t, err)
	require.NotNil(t, masterNode0)

	// Test deriving master node for index 1
	masterNode1, err := phrase.DeriveMasterNode(1)
	require.NoError(t, err)
	require.NotNil(t, masterNode1)

	// Verify that different indices produce different nodes
	raw0, err := masterNode0.RawSeed(), nil
	require.NoError(t, err)
	raw1, err := masterNode1.RawSeed(), nil
	require.NoError(t, err)
	require.NotEqual(t, raw0, raw1)
}

func TestDeriveKeysFromMasterNode(t *testing.T) {
	phrase, err := NewMnemonicGenerator().WithWordCount(12)
	require.NoError(t, err)

	// Get master node
	masterNode, err := phrase.DeriveMasterNode(0)
	require.NoError(t, err)

	// Derive keys from master node
	result, err := DeriveKeysFromMasterNode(masterNode)
	require.NoError(t, err)
	require.NotNil(t, result.MasterKey)
	require.NotNil(t, result.Identity)

	// Verify the keys can sign and verify
	testData := []byte("test data for signing")
	
	// Test master key
	masterSig, err := result.MasterKey.Sign(testData)
	require.NoError(t, err)
	verified, err := result.MasterKey.GetPublic().Verify(testData, masterSig)
	require.NoError(t, err)
	require.True(t, verified)

	// Test identity key
	identitySig, err := result.Identity.Sign(testData)
	require.NoError(t, err)
	verified, err = result.Identity.GetPublic().Verify(testData, identitySig)
	require.NoError(t, err)
	require.True(t, verified)
}

func TestMasterNodeDerivationConsistency(t *testing.T) {
	// Use a fixed mnemonic for consistency test
	var phrase Mnemonic = "tag volcano eight thank tide danger coast health above argue embrace heavy"

	// Derive using the traditional method
	traditionalResult, err := phrase.DeriveKeys(0)
	require.NoError(t, err)

	// Derive using the new master node method
	masterNode, err := phrase.DeriveMasterNode(0)
	require.NoError(t, err)
	newMethodResult, err := DeriveKeysFromMasterNode(masterNode)
	require.NoError(t, err)

	// Verify that both methods produce the same master key
	require.True(t, traditionalResult.MasterKey.Equals(newMethodResult.MasterKey))
	
	// Verify that both methods produce the same identity
	require.True(t, traditionalResult.Identity.Equals(newMethodResult.Identity))
}

func TestBackwardCompatibility(t *testing.T) {
	// Test that existing functionality still works
	phrase, err := NewMnemonicGenerator().WithWordCount(12)
	require.NoError(t, err)
	
	// Test traditional DeriveKeys method
	result, err := phrase.DeriveKeys(0)
	require.NoError(t, err)
	require.NotNil(t, result.MasterKey)
	require.NotNil(t, result.Identity)
	require.NotNil(t, result.OldAccountKey)
	require.NotNil(t, result.MasterNode)
	
	// Verify Ethereum identity is still derived
	publicKey := result.EthereumIdentity.Public()
	_, ok := publicKey.(*ecdsa.PublicKey)
	require.True(t, ok)
}

func TestMasterNodeSerialization(t *testing.T) {
	// Generate a test mnemonic
	phrase, err := NewMnemonicGenerator().WithWordCount(12)
	require.NoError(t, err)
	
	// Derive a master node
	originalNode, err := phrase.DeriveMasterNode(0)
	require.NoError(t, err)
	
	// Serialize the node using slip10's MarshalBinary
	serialized, err := originalNode.MarshalBinary()
	require.NoError(t, err)
	require.Len(t, serialized, 64) // Should be exactly 64 bytes
	
	// Deserialize the node using slip10's UnmarshalNode
	deserializedNode, err := slip10.UnmarshalNode(serialized)
	require.NoError(t, err)
	
	// Verify the deserialized node produces the same keys
	originalResult, err := DeriveKeysFromMasterNode(originalNode)
	require.NoError(t, err)
	
	deserializedResult, err := DeriveKeysFromMasterNode(deserializedNode)
	require.NoError(t, err)
	
	// Compare master keys
	require.True(t, originalResult.MasterKey.Equals(deserializedResult.MasterKey))
	
	// Compare identity keys
	require.True(t, originalResult.Identity.Equals(deserializedResult.Identity))
	
	// Verify the deserialized node can still derive child keys
	childNode, err := deserializedNode.Derive(slip10.FirstHardenedIndex + 1)
	require.NoError(t, err)
	require.NotNil(t, childNode)
}

func TestMasterNodeSerializationConsistency(t *testing.T) {
	// Use a fixed mnemonic for consistency
	var phrase Mnemonic = "tag volcano eight thank tide danger coast health above argue embrace heavy"
	
	// Derive master node at index 0
	node0, err := phrase.DeriveMasterNode(0)
	require.NoError(t, err)
	
	// Serialize and deserialize using slip10 methods
	serialized0, err := node0.MarshalBinary()
	require.NoError(t, err)
	
	deserialized0, err := slip10.UnmarshalNode(serialized0)
	require.NoError(t, err)
	
	// Derive a child from both original and deserialized
	originalChild, err := node0.Derive(slip10.FirstHardenedIndex)
	require.NoError(t, err)
	
	deserializedChild, err := deserialized0.Derive(slip10.FirstHardenedIndex)
	require.NoError(t, err)
	
	// Verify both children produce the same key
	originalKey, err := genKey(originalChild)
	require.NoError(t, err)
	
	deserializedKey, err := genKey(deserializedChild)
	require.NoError(t, err)
	
	require.True(t, originalKey.Equals(deserializedKey))
}

func TestInvalidSerialization(t *testing.T) {
	// Test with invalid data lengths
	testCases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"too short", make([]byte, 32)},
		{"too long", make([]byte, 128)},
		{"almost correct", make([]byte, 63)},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := slip10.UnmarshalNode(tc.data)
			require.Error(t, err)
		})
	}
}

package crypto

import (
	"crypto/ecdsa"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/anyproto/go-slip10"
	"github.com/stretchr/testify/require"
)

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
	_, ok := publicKey.(*ecdsa.PublicKey)
	require.Equal(t, true, ok)
	//ethAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	//require.Equal(t, common.HexToAddress("0xC49926C4124cEe1cbA0Ea94Ea31a6c12318df947"), ethAddress)
}

/*
// this test was using go-ethereum codebase which is huge
// we got rid of it
func TestMnemonic_ethereumKeyFromMnemonic(t *testing.T) {
	var badPphrase Mnemonic = "tag volcano"
	_, err := badPphrase.ethereumKeyFromMnemonic(0, defaultEthereumDerivation)
	require.Error(t, err)

	// good
	var phrase Mnemonic = "tag volcano eight thank tide danger coast health above argue embrace heavy"

	pk, err := phrase.ethereumKeyFromMnemonic(0, defaultEthereumDerivation)
	require.NoError(t, err)

	// what wallet.PrivateKeyHex(account) does
	bytes := crypto.FromECDSA(pk)
	pkStr := hexutil.Encode(bytes)[2:]
	require.Equal(t, "63e21d10fd50155dbba0e7d3f7431a400b84b4c2ac1ee38872f82448fe3ecfb9", pkStr)

	pk, err = phrase.ethereumKeyFromMnemonic(1, defaultEthereumDerivation)
	require.NoError(t, err)

	bytes = crypto.FromECDSA(pk)
	pkStr = hexutil.Encode(bytes)[2:]
	require.Equal(t, "b31048b0aa87649bdb9016c0ee28c788ddfc45e52cd71cc0da08c47cb4390ae7", pkStr)
}*/

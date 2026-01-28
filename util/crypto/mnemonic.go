package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"errors"
	"fmt"

	"github.com/anyproto/go-bip39"
	"github.com/anyproto/go-slip10"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"

	"github.com/anyproto/any-sync/util/ethereum/accounts"
)

var (
	ErrInvalidWordCount = errors.New("error invalid word count for mnemonic")
	ErrInvalidMnemonic  = errors.New("error invalid mnemonic")
)

const (
	// https://github.com/satoshilabs/slips/blob/master/slip-0044.md
	anytypeAccountNewPrefix = "m/44'/2046'"

	// https://github.com/bitcoin/bips/blob/master/bip-0044.mediawiki
	// standard Ethereum path (MetaMask, MyEtherWallet, etc) is
	// "m/44'/60'/0'/0/0" for first account
	// "m/44'/60'/0'/0/1" for second account, etc
	defaultEthereumDerivation = "m/44'/60'/0'/0"
)

type DerivationResult struct {
	// m/44'/code'/index'
	MasterKey PrivKey
	// m/44'/code'/index'/0'
	Identity   PrivKey
	MasterNode slip10.Node

	// Anytype uses ED25519
	// Ethereum and Bitcoin use ECDSA secp256k1 elliptic curves
	//
	// this key is used to sign ethereum transactions to use Any Naming Service
	// same mnemonic/seed phrase is used as for AnyType identity
	// m/44'/60'/0'/0/index
	EthereumIdentity ecdsa.PrivateKey
}

// DeriveKeysFromMasterNode derives master key and identity from a master node
// The master node should be at path m/44'/2046'/index'
func DeriveKeysFromMasterNode(masterNode slip10.Node) (res DerivationResult, err error) {
	res.MasterNode = masterNode

	// Derive master key from the node
	res.MasterKey, err = genKey(masterNode)
	if err != nil {
		return
	}

	// Derive identity at m/44'/2046'/index'/0'
	identityNode, err := masterNode.Derive(slip10.FirstHardenedIndex)
	if err != nil {
		return
	}
	res.Identity, err = genKey(identityNode)

	return
}

// DeriveMasterNode derives a master node at the specified index
// Returns the node at path m/44'/2046'/index'
func (m Mnemonic) DeriveMasterNode(index uint32) (masterNode slip10.Node, err error) {
	seed, err := m.Seed()
	if err != nil {
		return
	}

	prefixNode, err := slip10.DeriveForPath(anytypeAccountNewPrefix, seed)
	if err != nil {
		return
	}

	// m/44'/2046'/index'
	masterNode, err = prefixNode.Derive(slip10.FirstHardenedIndex + index)
	return
}

// DeriveMasterNodeFromSeed derives a master node from a seed at the specified index
// This creates a node at path m/44'/2046'/index'
func DeriveMasterNodeFromSeed(seed []byte, index uint32) (masterNode slip10.Node, err error) {
	prefixNode, err := slip10.DeriveForPath(anytypeAccountNewPrefix, seed)
	if err != nil {
		return
	}

	// m/44'/2046'/index'
	masterNode, err = prefixNode.Derive(slip10.FirstHardenedIndex + index)
	return
}

type MnemonicGenerator struct {
	mnemonic string
}

func NewMnemonicGenerator() MnemonicGenerator {
	return MnemonicGenerator{}
}

type Mnemonic string

func (g MnemonicGenerator) WithWordCount(wc int) (Mnemonic, error) {
	size := 0
	switch wc {
	case 12:
		size = 128
	case 15:
		size = 160
	case 18:
		size = 192
	case 21:
		size = 224
	case 24:
		size = 256
	default:
		return "", ErrInvalidWordCount
	}
	return g.WithRandomEntropy(size)
}

func (g MnemonicGenerator) WithRandomEntropy(size int) (Mnemonic, error) {
	entropy, err := bip39.NewEntropy(size)
	if err != nil {
		return "", err
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", err
	}
	return Mnemonic(mnemonic), nil
}

func (g MnemonicGenerator) WithEntropy(b []byte) (Mnemonic, error) {
	mnemonic, err := bip39.NewMnemonic(b)
	if err != nil {
		return "", err
	}
	return Mnemonic(mnemonic), nil
}

func (m Mnemonic) deriveForPath(onlyMaster bool, index uint32, path string) (res DerivationResult, err error) {
	seed, err := m.Seed()
	if err != nil {
		return
	}
	prefixNode, err := slip10.DeriveForPath(path, seed)
	if err != nil {
		return
	}
	// m/44'/code'/index'
	res.MasterNode, err = prefixNode.Derive(slip10.FirstHardenedIndex + index)
	if err != nil {
		return
	}

	if onlyMaster {
		// Only derive the master key
		res.MasterKey, err = genKey(res.MasterNode)
		return
	}

	// Use the public method to derive both master key and identity
	return DeriveKeysFromMasterNode(res.MasterNode)
}

func (m Mnemonic) DeriveKeys(index uint32) (res DerivationResult, err error) {
	// Derive master node using the new public method
	masterNode, err := m.DeriveMasterNode(index)
	if err != nil {
		return
	}

	// Derive keys from master node using the public method
	res, err = DeriveKeysFromMasterNode(masterNode)
	if err != nil {
		return
	}

	// Derive ethereum key
	pk, err := m.ethereumKeyFromMnemonic(index, defaultEthereumDerivation)
	if err != nil {
		return
	}
	res.EthereumIdentity = *pk

	return
}

func (m Mnemonic) Seed() ([]byte, error) {
	seed, err := bip39.NewSeedWithErrorChecking(string(m), "")
	if err != nil {
		if err == bip39.ErrInvalidMnemonic {
			return nil, ErrInvalidMnemonic
		}
		return nil, err
	}
	return seed, nil
}

func (m Mnemonic) Bytes() ([]byte, error) {
	return bip39.MnemonicToByteArray(string(m), true)
}

func genKey(node slip10.Node) (key PrivKey, err error) {
	reader := bytes.NewReader(node.RawSeed())
	key, _, err = GenerateEd25519Key(reader)
	return
}

func derivePrivateKey(masterKey *hdkeychain.ExtendedKey, path accounts.DerivationPath) (*ecdsa.PrivateKey, error) {
	var err error
	key := masterKey
	for _, n := range path {
		key, err = key.DeriveNonStandard(n)

		if err != nil {
			return nil, err
		}
	}

	privateKey, err := key.ECPrivKey()
	privateKeyECDSA := privateKey.ToECDSA()
	if err != nil {
		return nil, err
	}

	return privateKeyECDSA, nil
}

func (m Mnemonic) ethereumKeyFromMnemonic(index uint32, path string) (pk *ecdsa.PrivateKey, err error) {
	seed, err := m.Seed()
	if err != nil {
		return
	}

	masterKey, err := hdkeychain.NewMaster(seed, &chaincfg.MainNetParams)
	if err != nil {
		return nil, err
	}

	// m/44'/60'/0'/0/0 for first account
	// m/44'/60'/0'/0/1 for second account, etc
	fullPath := fmt.Sprintf("%s/%d", path, index)
	p, err := accounts.ParseDerivationPath(fullPath)
	if err != nil {
		panic(err)
	}

	pk, err = derivePrivateKey(masterKey, p)
	if err != nil {
		return nil, err
	}

	return pk, nil
}

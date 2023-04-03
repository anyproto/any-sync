package crypto

import (
	"bytes"
	"errors"
	"github.com/anytypeio/go-slip10"
	"github.com/tyler-smith/go-bip39"
)

var (
	ErrInvalidWordCount = errors.New("error invalid word count for mnemonic")
	ErrInvalidMnemonic  = errors.New("error invalid mnemonic")
)

const (
	anytypeAccountOldPrefix = "m/44'/607'"
	// TODO: https://github.com/satoshilabs/slips/pull/1528
	anytypeAccountNewPrefix = "m/44'/2046'"
)

type DerivationResult struct {
	MasterKey     PrivKey
	Identity      PrivKey
	OldAccountKey PrivKey
	MasterNode    slip10.Node
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
	res.MasterKey, err = genKey(res.MasterNode)
	if err != nil || onlyMaster {
		return
	}
	// m/44'/code'/index'/0'
	identityNode, err := res.MasterNode.Derive(slip10.FirstHardenedIndex)
	if err != nil {
		return
	}
	res.Identity, err = genKey(identityNode)
	return
}

func (m Mnemonic) DeriveKeys(index uint32) (res DerivationResult, err error) {
	oldRes, err := m.deriveForPath(true, index, anytypeAccountOldPrefix)
	if err != nil {
		return
	}
	res, err = m.deriveForPath(false, index, anytypeAccountNewPrefix)
	if err != nil {
		return
	}
	res.OldAccountKey = oldRes.MasterKey
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

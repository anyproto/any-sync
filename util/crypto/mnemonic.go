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

type MnemonicGenerator struct {
	mnemonic string
}

func NewMnemonicGenerator() MnemonicGenerator {
	return MnemonicGenerator{}
}

type Mnemonic string

func (g *MnemonicGenerator) WithWordCount(wc int) (Mnemonic, error) {
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
	return g.WithWordCount(size)
}

func (g *MnemonicGenerator) WithRandomEntropy(size int) (Mnemonic, error) {
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

func (g *MnemonicGenerator) WithEntropy(b []byte) (Mnemonic, error) {
	mnemonic, err := bip39.NewMnemonic(b)
	if err != nil {
		return "", err
	}
	return Mnemonic(mnemonic), nil
}

func (m Mnemonic) DeriveEd25519Key(index int) (PrivKey, error) {
	seed, err := bip39.NewSeedWithErrorChecking(string(m), "")
	if err != nil {
		if err == bip39.ErrInvalidMnemonic {
			return nil, ErrInvalidMnemonic
		}
		return nil, err
	}
	masterKey, err := slip10.DeriveForPath(AnytypeAccountPrefix, seed)
	if err != nil {
		return nil, err
	}

	key, err := masterKey.Derive(slip10.FirstHardenedIndex + uint32(index))
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(key.RawSeed())
	privKey, _, err := GenerateEd25519Key(reader)

	return privKey, err
}

func (m Mnemonic) Bytes() ([]byte, error) {
	return bip39.MnemonicToByteArray(string(m), true)
}

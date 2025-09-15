package crypto

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"io"

	"filippo.io/edwards25519"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/nacl/box"
)

var ErrX25519DecryptionFailed = errors.New("failed decryption with x25519 key")

// Ed25519PublicKeyToCurve25519 converts an Ed25519 public key to a Curve25519 public key
func Ed25519PublicKeyToCurve25519(pk ed25519.PublicKey) []byte {
	// Unmarshalling public key into edwards curve point
	epk, err := (&edwards25519.Point{}).SetBytes(pk)
	if err != nil {
		panic(err)
	}
	// converting to curve25519 (see here for more details https://github.com/golang/go/issues/20504)
	return epk.BytesMontgomery()
}

// ISC License
//
// Copyright (c) 2013-2020
// Frank Denis <j at pureftpd dot org>
//
// Permission to use, copy, modify, and/or distribute this software for any
// purpose with or without fee is hereby granted, provided that the above
// copyright notice and this permission notice appear in all copies.
//
// THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
// WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
// ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
// WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
// ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
// OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
// https://github.com/jedisct1/libsodium/blob/master/src/libsodium/crypto_sign/ed25519/ref10/keypair.c#L69-L83

// Ed25519PrivateKeyToCurve25519 converts an Ed25519 private key to a Curve25519 private key
// This code is originally taken from here https://github.com/jorrizza/ed2curve25519/blob/master/ed2curve25519.go
func Ed25519PrivateKeyToCurve25519(pk ed25519.PrivateKey) []byte {
	h := sha512.New()
	h.Write(pk.Seed())
	out := h.Sum(nil)

	// used in libsodium
	out[0] &= 248
	out[31] &= 127
	out[31] |= 64

	return out[:curve25519.ScalarSize]
}

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

//

func buildSymmetricContext(a, b []byte) []byte {
	// add label here so that we can distinguish from other potential implementations
	label := []byte("joined-identity-v1")
	if bytes.Compare(a, b) <= 0 {
		return append(append([]byte{}, label...), append(a, b...)...)
	}
	return append(append([]byte{}, label...), append(b, a...)...)
}

func GenerateSharedKey(aSk PrivKey, bPk PubKey) (PrivKey, error) {
	skRaw, err := aSk.Raw()
	if err != nil {
		return nil, err
	}
	skCurve := Ed25519PrivateKeyToCurve25519(ed25519.PrivateKey(skRaw))

	pkRaw, err := bPk.Raw()
	if err != nil {
		return nil, err
	}

	pkCurve := Ed25519PublicKeyToCurve25519(pkRaw)

	shared, err := curve25519.X25519(skCurve, pkCurve[:])
	if err != nil {
		return nil, err
	}

	aPkRaw, err := aSk.GetPublic().Raw()
	if err != nil {
		return nil, err
	}

	bPkRaw, err := bPk.Raw()
	if err != nil {
		return nil, err
	}

	ctx := buildSymmetricContext(aPkRaw, bPkRaw)
	h := hkdf.New(sha256.New, shared, nil, ctx)
	var seed [32]byte
	if _, err := io.ReadFull(h, seed[:]); err != nil {
		return nil, err
	}

	jointPriv := ed25519.NewKeyFromSeed(seed[:])

	return NewEd25519PrivKey(jointPriv), nil
}

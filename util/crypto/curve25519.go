package crypto

import (
	"crypto/ed25519"
	"crypto/sha512"
	"filippo.io/edwards25519"
	"golang.org/x/crypto/curve25519"
)

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

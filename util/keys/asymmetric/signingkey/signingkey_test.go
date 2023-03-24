package signingkey

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"testing"
)

func Test(t *testing.T) {
	pubKey, privKey, _ := ed25519.GenerateKey(rand.Reader)
	msg := []byte("some stuffsafeesafujeaiofjoeai joaij fioaj iofaj oifaj foiajio fjao jo")
	enc := EncryptWithEd25519(pubKey, msg)
	dec := DecryptWithEd25519(pubKey, privKey, enc)
	fmt.Println(string(enc), string(dec))
}

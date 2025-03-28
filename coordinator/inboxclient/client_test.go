package inboxclient

import (
	"testing"

	"github.com/anyproto/any-sync/util/crypto"
	"github.com/stretchr/testify/assert"
)

func TestInbox_CryptoTest(t *testing.T) {
	privKey, pubKey, _ := crypto.GenerateRandomEd25519KeyPair()

	t.Run("test encrypt/decrypt", func(t *testing.T) {
		body := []byte("hello")
		encrypted, _ := pubKey.Encrypt(body)
		decrypted, _ := privKey.Decrypt(encrypted)
		assert.Equal(t, body, decrypted)
	})
}

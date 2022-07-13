package keys

type EncryptionPrivKey interface {
	Key

	Decrypt([]byte) ([]byte, error)
	GetPublic() EncryptionPubKey
}

type EncryptionPubKey interface {
	Key

	Encrypt(data []byte) ([]byte, error)
}

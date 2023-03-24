package crypto

type KeyStorage interface {
	PubKeyFromProto(protoBytes []byte) (PubKey, error)
}

func NewKeyStorage() KeyStorage {
	return nil
}

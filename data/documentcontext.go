package data

import "github.com/anytypeio/go-anytype-infrastructure-experiments/data/threadmodels"

type documentContext struct {
	aclTree       *Tree
	fullTree      *Tree
	identity      string
	encryptionKey threadmodels.EncryptionPrivKey
	decoder       threadmodels.SigningPubKeyDecoder
	aclState      *ACLState
	docState      DocumentState
	changeCache   map[string]*Change
	identityKeys  map[string]threadmodels.SigningPubKey
}

func newDocumentContext(
	identity string,
	encryptionKey threadmodels.EncryptionPrivKey,
	decoder threadmodels.SigningPubKeyDecoder) *documentContext {
	return &documentContext{
		aclTree:       &Tree{},
		fullTree:      &Tree{},
		identity:      identity,
		encryptionKey: encryptionKey,
		decoder:       decoder,
		aclState:      nil,
		docState:      nil,
		changeCache:   make(map[string]*Change),
		identityKeys:  make(map[string]threadmodels.SigningPubKey),
	}
}

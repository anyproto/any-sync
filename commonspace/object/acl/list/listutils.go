package list

import (
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/crypto"
)

type StorageProvider func(root *consensusproto.RawRecordWithId) (Storage, error)

func NewInMemoryDerivedAcl(spaceId string, keys *accountdata.AccountKeys) (AclList, error) {
	return newInMemoryDerivedAclMetadata(spaceId, keys, []byte("metadata"))
}

func newAclWithStoreProvider(root *consensusproto.RawRecordWithId, keys *accountdata.AccountKeys, storeProvider StorageProvider) (AclList, error) {
	storage, err := storeProvider(root)
	if err != nil {
		return nil, err
	}
	return BuildAclListWithIdentity(keys, storage, recordverifier.NewValidateFull())
}

func newDerivedAclWithStoreProvider(spaceId string, keys *accountdata.AccountKeys, metadata []byte, storeProvider StorageProvider) (AclList, error) {
	root, err := buildDerivedRoot(spaceId, keys, metadata)
	if err != nil {
		return nil, err
	}
	return newAclWithStoreProvider(root, keys, storeProvider)
}

func newInMemoryDerivedAclMetadata(spaceId string, keys *accountdata.AccountKeys, metadata []byte) (AclList, error) {
	root, err := buildDerivedRoot(spaceId, keys, metadata)
	if err != nil {
		return nil, err
	}
	return newInMemoryAclWithRoot(keys, root)
}

func newInMemoryAclWithRoot(keys *accountdata.AccountKeys, root *consensusproto.RawRecordWithId) (AclList, error) {
	st, err := NewInMemoryStorage(root.Id, []*consensusproto.RawRecordWithId{
		root,
	})
	if err != nil {
		return nil, err
	}
	return BuildAclListWithIdentity(keys, st, recordverifier.NewValidateFull())
}

func buildDerivedRoot(spaceId string, keys *accountdata.AccountKeys, metadata []byte) (root *consensusproto.RawRecordWithId, err error) {
	builder := NewAclRecordBuilder("", crypto.NewKeyStorage(), keys, recordverifier.NewValidateFull())
	masterKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	if err != nil {
		return nil, err
	}
	newReadKey := crypto.NewAES()
	privKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	if err != nil {
		return nil, err
	}
	return builder.BuildRoot(RootContent{
		PrivKey:   keys.SignKey,
		SpaceId:   spaceId,
		MasterKey: masterKey,
		Change: ReadKeyChangePayload{
			MetadataKey: privKey,
			ReadKey:     newReadKey,
		},
		Metadata: metadata,
	})
}

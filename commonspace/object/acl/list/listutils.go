package list

import (
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/crypto"
)

func NewInMemoryDerivedAcl(spaceId string, keys *accountdata.AccountKeys) (AclList, error) {
	return newInMemoryDerivedAclMetadata(spaceId, keys, []byte("metadata"))
}

func newInMemoryDerivedAclMetadata(spaceId string, keys *accountdata.AccountKeys, metadata []byte) (AclList, error) {
	builder := NewAclRecordBuilder("", crypto.NewKeyStorage(), keys, NoOpAcceptorVerifier{})
	masterKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	if err != nil {
		return nil, err
	}
	newReadKey := crypto.NewAES()
	privKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	if err != nil {
		return nil, err
	}
	root, err := builder.BuildRoot(RootContent{
		PrivKey:   keys.SignKey,
		SpaceId:   spaceId,
		MasterKey: masterKey,
		Change: ReadKeyChangePayload{
			MetadataKey: privKey,
			ReadKey:     newReadKey,
		},
		Metadata: metadata,
	})
	if err != nil {
		return nil, err
	}
	st, err := NewInMemoryStorage(root.Id, []*consensusproto.RawRecordWithId{
		root,
	})
	if err != nil {
		return nil, err
	}
	return BuildAclListWithIdentity(keys, st, NoOpAcceptorVerifier{})
}

func newInMemoryAclWithRoot(keys *accountdata.AccountKeys, root *consensusproto.RawRecordWithId) (AclList, error) {
	st, err := NewInMemoryStorage(root.Id, []*consensusproto.RawRecordWithId{
		root,
	})
	if err != nil {
		return nil, err
	}
	return BuildAclListWithIdentity(keys, st, NoOpAcceptorVerifier{})
}

package list

import (
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/liststorage"
	"github.com/anyproto/any-sync/util/crypto"
)

func NewTestDerivedAcl(spaceId string, keys *accountdata.AccountKeys) (AclList, error) {
	builder := NewAclRecordBuilder("", crypto.NewKeyStorage(), keys)
	masterKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	if err != nil {
		return nil, err
	}
	root, err := builder.BuildRoot(RootContent{
		PrivKey:   keys.SignKey,
		SpaceId:   spaceId,
		MasterKey: masterKey,
	})
	if err != nil {
		return nil, err
	}
	st, err := liststorage.NewInMemoryAclListStorage(root.Id, []*aclrecordproto.RawAclRecordWithId{
		root,
	})
	if err != nil {
		return nil, err
	}
	return BuildAclListWithIdentity(keys, st, NoOpAcceptorVerifier{})
}

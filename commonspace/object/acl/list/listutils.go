package list

import (
	"fmt"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/liststorage"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/crypto"
)

func NewTestDerivedAcl(spaceId string, keys *accountdata.AccountKeys) (AclList, error) {
	return NewTestDerivedAclMetadata(spaceId, keys, []byte("metadata"))
}

func NewTestDerivedAclMetadata(spaceId string, keys *accountdata.AccountKeys, metadata []byte) (AclList, error) {
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
	st, err := liststorage.NewInMemoryAclListStorage(root.Id, []*consensusproto.RawRecordWithId{
		root,
	})
	if err != nil {
		return nil, err
	}
	return BuildAclListWithIdentity(keys, st, NoOpAcceptorVerifier{})
}

func NewTestAclWithRoot(keys *accountdata.AccountKeys, root *consensusproto.RawRecordWithId) (AclList, error) {
	st, err := liststorage.NewInMemoryAclListStorage(root.Id, []*consensusproto.RawRecordWithId{
		root,
	})
	if err != nil {
		return nil, err
	}
	return BuildAclListWithIdentity(keys, st, NoOpAcceptorVerifier{})
}

func NewTestAclStateWithUsers(numWriters, numReaders, numInvites int) *AclState {
	st := &AclState{
		keys:            make(map[string]AclKeys),
		accountStates:   make(map[string]AccountState),
		inviteKeys:      make(map[string]crypto.PubKey),
		requestRecords:  make(map[string]RequestRecord),
		pendingRequests: make(map[string]string),
		keyStore:        crypto.NewKeyStorage(),
	}
	for i := 0; i < numWriters; i++ {
		st.accountStates[fmt.Sprint("w", i)] = AccountState{
			Permissions: AclPermissionsWriter,
			Status:      StatusActive,
		}
	}
	for i := 0; i < numReaders; i++ {
		st.accountStates[fmt.Sprint("r", i)] = AccountState{
			Permissions: AclPermissionsReader,
			Status:      StatusActive,
		}
	}
	for i := 0; i < numInvites; i++ {
		st.inviteKeys[fmt.Sprint("r", i)] = nil
	}
	return st
}

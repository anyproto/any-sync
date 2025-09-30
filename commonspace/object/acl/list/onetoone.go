package list

import (
	"bytes"
	"fmt"

	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/util/crypto"
)

func (st *AclState) setOneToOneAcl(rootId string, root *aclrecordproto.AclRoot) error {
	foundMe, err := st.findMeAndValidateOneToOne(root)
	if err != nil {
		return err
	}

	if foundMe {
		err := st.deriveOneToOneKeys(rootId, root)
		if err != nil {
			return err
		}
	}

	sharedPk, err := crypto.UnmarshalEd25519PublicKeyProto(root.OneToOneInfo.Owner)
	if err != nil {
		return err
	}

	st.accountStates[string(sharedPk.Storage())] = AccountState{
		PubKey:          sharedPk,
		Permissions:     AclPermissionsOwner,
		Status:          StatusActive,
		RequestMetadata: []byte{},
		KeyRecordId:     rootId,
		PermissionChanges: []PermissionChange{
			{
				RecordId:   rootId,
				Permission: AclPermissionsOwner,
			},
		},
	}

	for _, writer := range root.OneToOneInfo.Writers {
		var writerPk crypto.PubKey
		writerPk, err = crypto.UnmarshalEd25519PublicKeyProto(writer)
		if err != nil {
			return err
		}

		st.accountStates[string(writerPk.Storage())] = AccountState{
			PubKey:          writerPk,
			Permissions:     AclPermissionsWriter,
			Status:          StatusActive,
			RequestMetadata: []byte{},
			KeyRecordId:     rootId,
			PermissionChanges: []PermissionChange{
				{
					RecordId:   rootId,
					Permission: AclPermissionsWriter,
				},
			},
		}
	}

	st.readKeyChanges = []string{rootId}
	st.lastRecordId = rootId
	st.isOneToOne = true
	return nil

}

// returns true if current account identity is in Writers
// it will be false for node account
func (st *AclState) findMeAndValidateOneToOne(root *aclrecordproto.AclRoot) (foundMe bool, err error) {
	if len(root.OneToOneInfo.Writers) != 2 {
		return false, fmt.Errorf("findMeAndValidateOneToOne: AclRoot should have exactly two Writers, but got %d", len(root.OneToOneInfo.Writers))
	}
	if root.OneToOneInfo.Owner == nil {
		return false, fmt.Errorf("findMeAndValidateOneToOne: AclRoot Owner is empty")
	}

	myPubKeyBytes, err := st.key.GetPublic().Marshall()
	if err != nil {
		err = fmt.Errorf("findMeAndValidateOneToOne: error Marshal() st.key: %w", err)
		return
	}

	for _, writerBytes := range root.OneToOneInfo.Writers {
		if bytes.Equal(myPubKeyBytes, writerBytes) {
			foundMe = true
		}
	}

	return
}

// Derives my and bob shared secret key and sets rootId AclState keys
func (st *AclState) deriveOneToOneKeys(rootId string, root *aclrecordproto.AclRoot) (err error) {
	var bobPubKeyBytes []byte
	myPubKeyBytes, err := st.key.GetPublic().Marshall()
	if err != nil {
		err = fmt.Errorf("deriveOneToOneKeys: error Marshal() st.key: %w", err)
		return
	}

	if !bytes.Equal(myPubKeyBytes, root.OneToOneInfo.Writers[0]) {
		bobPubKeyBytes = root.OneToOneInfo.Writers[0]
	} else {
		bobPubKeyBytes = root.OneToOneInfo.Writers[1]
	}

	bobPubKey, err := crypto.UnmarshalEd25519PublicKeyProto(bobPubKeyBytes)
	if err != nil {
		err = fmt.Errorf("deriveOneToOneKeys: error Unmarshal(bobPubKeyBytes): %w", err)
		return
	}

	sharedKey, err := crypto.GenerateSharedKey(st.key, bobPubKey, crypto.AnysyncOneToOneSpacePath)
	if err != nil {
		err = fmt.Errorf("deriveOneToOneKeys: error GenerateSharedKey: %w", err)
		return
	}

	sharedPkBytes, err := sharedKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	if !bytes.Equal(root.OneToOneInfo.Owner, sharedPkBytes) {
		err = fmt.Errorf("deriveOneToOneKeys: Owner pubkey != derived pubkey")
		return
	}

	sharedKeyBytes, err := sharedKey.Raw()
	if err != nil {
		err = fmt.Errorf("deriveOneToOneKeys: error getting sharedKey.Raw(): %w", err)
		return
	}

	readKey, err := crypto.DeriveSymmetricKey(sharedKeyBytes, crypto.AnysyncReadOneToOneSpacePath)
	if err != nil {
		err = fmt.Errorf("deriveOneToOneKeys: error DeriveSymmetricKey: %w", err)
		return
	}

	metadataSharedKey, err := crypto.GenerateSharedKey(st.key, bobPubKey, crypto.AnysyncMetadataOneToOnePath)
	if err != nil {
		err = fmt.Errorf("deriveOneToOneKeys: metadataSharedKey: %w", err)
		return
	}

	metadataSharedKeyBytes, err := metadataSharedKey.Raw()
	if err != nil {
		err = fmt.Errorf("deriveOneToOneKeys: metadataSharedKey.Raw(): %w", err)
		return
	}

	encMetadatKey, err := readKey.Encrypt(metadataSharedKeyBytes)
	if err != nil {
		err = fmt.Errorf("deriveOneToOneKeys: readKey.Encrypt(): %w", err)
		return
	}

	st.keys[rootId] = AclKeys{
		ReadKey:         readKey,
		MetadataPrivKey: metadataSharedKey,
		MetadataPubKey:  metadataSharedKey.GetPublic(),
		encMetadatKey:   encMetadatKey,
	}

	return
}

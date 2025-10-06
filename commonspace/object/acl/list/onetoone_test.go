package list

import (
	"crypto/rand"
	"errors"
	"testing"

	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAclBuild_OneToOne(t *testing.T) {
	// 2
	t.Run("BuildOneToOneRoot", func(t *testing.T) {

	})
	// 1
	t.Run("ApplyRecord to onetoone returns error", func(t *testing.T) {

	})
	// 2
	t.Run("applyChangeData, no decrypt if onetoone (TODO: decrypt when whe have metadata after inbox)", func(t *testing.T) {

	})
	// 1
	t.Run("state.IsOneToOne", func(t *testing.T) {
		// secretKey, _, _ := crypto.GenerateEd25519Key(rand.Reader)
		// root := &aclrecordproto.AclRoot{
		// 	OneToOneInfo: &aclrecordproto.AclOneToOneInfo{
		// 		Owner:   []byte{1, 0},
		// 		Writers: [][]byte{{1, 1, 0}, {1, 0, 0}},
		// 	},
		// }

		// record := &AclRecord{
		// 	Id:    "1",
		// 	Model: root,
		// }
		// var verifier recordverifier.AcceptorVerifier
		// st, err := newAclStateWithKeys(record, secretKey, verifier)
		// require.NoError(t, err)
		// assert.Equal(t, st.IsOneToOne(), false)

	})
	// 2
	t.Run("setOneToOneAcl", func(t *testing.T) {

	})
	// 1

	t.Run("findMeAndValidateOneToOne doesn't findMe in root", func(t *testing.T) {
		key, _, _ := crypto.GenerateEd25519Key(rand.Reader)
		root := &aclrecordproto.AclRoot{
			OneToOneInfo: &aclrecordproto.AclOneToOneInfo{
				Owner:   []byte{1, 0},
				Writers: [][]byte{{1, 1, 0}, {1, 0, 0}},
			},
		}
		st := newTestAclStateWithKey(key)

		foundMe, err := st.findMeAndValidateOneToOne(root)
		require.NoError(t, err)
		assert.False(t, foundMe)
	})

	t.Run("findMeAndValidateOneToOne returns error if writers count is invalid", func(t *testing.T) {
		key, _, _ := crypto.GenerateEd25519Key(rand.Reader)
		root := &aclrecordproto.AclRoot{
			OneToOneInfo: &aclrecordproto.AclOneToOneInfo{
				Owner:   []byte{1},
				Writers: [][]byte{{1}},
			},
		}
		st := newTestAclStateWithKey(key)

		_, err := st.findMeAndValidateOneToOne(root)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exactly two Writers")
	})

	t.Run("findMeAndValidateOneToOne returns error if owner is empty", func(t *testing.T) {
		key, _, _ := crypto.GenerateEd25519Key(rand.Reader)
		root := &aclrecordproto.AclRoot{
			OneToOneInfo: &aclrecordproto.AclOneToOneInfo{
				Writers: [][]byte{{1, 1, 0}, {1, 0, 0}},
			},
		}
		st := newTestAclStateWithKey(key)

		_, err := st.findMeAndValidateOneToOne(root)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Owner is empty")
	})

	t.Run("findMeAndValidateOneToOne finds me in writers", func(t *testing.T) {
		key, _, _ := crypto.GenerateEd25519Key(rand.Reader)
		myPubKeyBytes, err := key.GetPublic().Marshall()
		require.NoError(t, err)
		root := &aclrecordproto.AclRoot{
			OneToOneInfo: &aclrecordproto.AclOneToOneInfo{
				Owner:   []byte{1, 0},
				Writers: [][]byte{myPubKeyBytes, {2, 0}},
			},
		}
		st := newTestAclStateWithKey(key)

		foundMe, err := st.findMeAndValidateOneToOne(root)
		require.NoError(t, err)
		assert.True(t, foundMe)
	})

	t.Run("findMeAndValidateOneToOne returns marshal error", func(t *testing.T) {
		delegateKey, _, _ := crypto.GenerateEd25519Key(rand.Reader)
		root := &aclrecordproto.AclRoot{
			OneToOneInfo: &aclrecordproto.AclOneToOneInfo{
				Owner:   []byte{1, 0},
				Writers: [][]byte{{1, 1, 0}, {1, 0, 0}},
			},
		}
		errKey := errPrivKey{
			PrivKey: delegateKey,
			errPub:  delegateKey.GetPublic(),
		}
		st := newTestAclStateWithKey(errKey)

		_, err := st.findMeAndValidateOneToOne(root)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error Marshal() st.key")
	})
	// 1
	t.Run("deriveOneToOneKeys", func(t *testing.T) {

	})

}

func newTestAclStateWithKey(key crypto.PrivKey) *AclState {
	return &AclState{
		id:              "id1",
		key:             key,
		pubKey:          key.GetPublic(),
		keys:            make(map[string]AclKeys),
		accountStates:   make(map[string]AccountState),
		invites:         make(map[string]Invite),
		requestRecords:  make(map[string]RequestRecord),
		pendingRequests: make(map[string]string),
		keyStore:        crypto.NewKeyStorage(),
	}
}

type errPrivKey struct {
	crypto.PrivKey
	errPub crypto.PubKey
}

func (k errPrivKey) GetPublic() crypto.PubKey {
	return errPubKey{PubKey: k.errPub}
}

type errPubKey struct {
	crypto.PubKey
}

func (errPubKey) Marshall() ([]byte, error) {
	return nil, errors.New("marshall failure")
}

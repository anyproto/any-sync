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

func TestAclBuildOneToOne_findMeAndValidate(t *testing.T) {
	t.Run("doesn't findMe in root", func(t *testing.T) {
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

	t.Run("returns error if writers count is invalid", func(t *testing.T) {
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

	t.Run("returns error if owner is empty", func(t *testing.T) {
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

	t.Run("finds me in writers", func(t *testing.T) {
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

	t.Run("returns marshal error", func(t *testing.T) {
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

}

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
		t.Run("returns false by default", func(t *testing.T) {
			key, _, _ := crypto.GenerateEd25519Key(rand.Reader)
			st := newTestAclStateWithKey(key)

			assert.False(t, st.IsOneToOne())
		})

		t.Run("returns true after setOneToOneAcl", func(t *testing.T) {
			myKey, _, _ := crypto.GenerateEd25519Key(rand.Reader)
			bobKey, _, _ := crypto.GenerateEd25519Key(rand.Reader)

			myPubKeyBytes, _ := myKey.GetPublic().Marshall()
			bobPubKeyBytes, _ := bobKey.GetPublic().Marshall()

			sharedKey, _ := crypto.GenerateSharedKey(myKey, bobKey.GetPublic(), crypto.AnysyncOneToOneSpacePath)
			ownerBytes, _ := sharedKey.GetPublic().Marshall()

			root := &aclrecordproto.AclRoot{
				OneToOneInfo: &aclrecordproto.AclOneToOneInfo{
					Owner:   ownerBytes,
					Writers: [][]byte{myPubKeyBytes, bobPubKeyBytes},
				},
			}

			st := newTestAclStateWithKey(myKey)
			err := st.setOneToOneAcl("rootId", root)
			require.NoError(t, err)

			assert.True(t, st.IsOneToOne())
		})

		t.Run("remains false if setOneToOneAcl fails", func(t *testing.T) {
			key, _, _ := crypto.GenerateEd25519Key(rand.Reader)
			root := &aclrecordproto.AclRoot{
				OneToOneInfo: &aclrecordproto.AclOneToOneInfo{
					Owner:   []byte{1, 0},
					Writers: [][]byte{{1, 1, 0}}, // invalid: only one writer
				},
			}

			st := newTestAclStateWithKey(key)
			err := st.setOneToOneAcl("rootId", root)
			require.Error(t, err)

			assert.False(t, st.IsOneToOne())
		})
	})
	// 2
	t.Run("setOneToOneAcl", func(t *testing.T) {
		t.Run("error from findMeAndValidateOneToOne", func(t *testing.T) {
			key, _, _ := crypto.GenerateEd25519Key(rand.Reader)
			root := &aclrecordproto.AclRoot{
				OneToOneInfo: &aclrecordproto.AclOneToOneInfo{
					Owner:   []byte{1, 0},
					Writers: [][]byte{{1, 1, 0}}, // invalid: only one writer
				},
			}

			st := newTestAclStateWithKey(key)
			err := st.setOneToOneAcl("rootId", root)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "exactly two Writers")
			assert.False(t, st.IsOneToOne())
		})

		t.Run("foundMe=true, deriveOneToOneKeys error", func(t *testing.T) {
			myKey, _, _ := crypto.GenerateEd25519Key(rand.Reader)
			myPubKeyBytes, _ := myKey.GetPublic().Marshall()

			root := &aclrecordproto.AclRoot{
				OneToOneInfo: &aclrecordproto.AclOneToOneInfo{
					Owner:   []byte{1, 2, 3}, // invalid owner
					Writers: [][]byte{myPubKeyBytes, {4, 5, 6}}, // invalid bob key
				},
			}

			st := newTestAclStateWithKey(myKey)
			err := st.setOneToOneAcl("rootId", root)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "error Unmarshal(bobPubKeyBytes)")
			assert.False(t, st.IsOneToOne())
		})

		t.Run("foundMe=true, success path", func(t *testing.T) {
			myKey, _, _ := crypto.GenerateEd25519Key(rand.Reader)
			bobKey, _, _ := crypto.GenerateEd25519Key(rand.Reader)

			myPubKeyBytes, _ := myKey.GetPublic().Marshall()
			bobPubKeyBytes, _ := bobKey.GetPublic().Marshall()

			sharedKey, _ := crypto.GenerateSharedKey(myKey, bobKey.GetPublic(), crypto.AnysyncOneToOneSpacePath)
			ownerBytes, _ := sharedKey.GetPublic().Marshall()

			root := &aclrecordproto.AclRoot{
				OneToOneInfo: &aclrecordproto.AclOneToOneInfo{
					Owner:   ownerBytes,
					Writers: [][]byte{myPubKeyBytes, bobPubKeyBytes},
				},
			}

			st := newTestAclStateWithKey(myKey)
			err := st.setOneToOneAcl("rootId", root)

			require.NoError(t, err)
			assert.True(t, st.IsOneToOne())
			assert.Contains(t, st.keys, "rootId")
			assert.Equal(t, []string{"rootId"}, st.readKeyChanges)
			assert.Equal(t, "rootId", st.lastRecordId)

			// Check owner account state
			ownerStorage := string(sharedKey.GetPublic().Storage())
			assert.Contains(t, st.accountStates, ownerStorage)
			ownerState := st.accountStates[ownerStorage]
			assert.Equal(t, sharedKey.GetPublic().Storage(), ownerState.PubKey.Storage())

			// Check writer account states
			myStorage := string(myKey.GetPublic().Storage())
			assert.Contains(t, st.accountStates, myStorage)
			bobStorage := string(bobKey.GetPublic().Storage())
			assert.Contains(t, st.accountStates, bobStorage)
		})

		t.Run("foundMe=false, success path", func(t *testing.T) {
			myKey, _, _ := crypto.GenerateEd25519Key(rand.Reader)
			writer1Key, _, _ := crypto.GenerateEd25519Key(rand.Reader)
			writer2Key, _, _ := crypto.GenerateEd25519Key(rand.Reader)

			writer1Bytes, _ := writer1Key.GetPublic().Marshall()
			writer2Bytes, _ := writer2Key.GetPublic().Marshall()

			sharedKey, _ := crypto.GenerateSharedKey(writer1Key, writer2Key.GetPublic(), crypto.AnysyncOneToOneSpacePath)
			ownerBytes, _ := sharedKey.GetPublic().Marshall()

			root := &aclrecordproto.AclRoot{
				OneToOneInfo: &aclrecordproto.AclOneToOneInfo{
					Owner:   ownerBytes,
					Writers: [][]byte{writer1Bytes, writer2Bytes},
				},
			}

			st := newTestAclStateWithKey(myKey)
			err := st.setOneToOneAcl("rootId", root)

			require.NoError(t, err)
			assert.True(t, st.IsOneToOne())
			assert.NotContains(t, st.keys, "rootId") // no keys derived since foundMe=false
			assert.Equal(t, []string{"rootId"}, st.readKeyChanges)
			assert.Equal(t, "rootId", st.lastRecordId)

			// Check account states are still created
			assert.Len(t, st.accountStates, 3) // owner + 2 writers
		})

		t.Run("error unmarshaling owner public key", func(t *testing.T) {
			writer1Key, _, _ := crypto.GenerateEd25519Key(rand.Reader)
			writer2Key, _, _ := crypto.GenerateEd25519Key(rand.Reader)

			writer1Bytes, _ := writer1Key.GetPublic().Marshall()
			writer2Bytes, _ := writer2Key.GetPublic().Marshall()

			root := &aclrecordproto.AclRoot{
				OneToOneInfo: &aclrecordproto.AclOneToOneInfo{
					Owner:   []byte{1, 2, 3}, // invalid owner key
					Writers: [][]byte{writer1Bytes, writer2Bytes},
				},
			}

			myKey, _, _ := crypto.GenerateEd25519Key(rand.Reader)
			st := newTestAclStateWithKey(myKey)
			err := st.setOneToOneAcl("rootId", root)

			require.Error(t, err)
			assert.False(t, st.IsOneToOne())
		})

		t.Run("error unmarshaling writer public key", func(t *testing.T) {
			writer1Key, _, _ := crypto.GenerateEd25519Key(rand.Reader)
			writer1Bytes, _ := writer1Key.GetPublic().Marshall()

			sharedKey, _, _ := crypto.GenerateEd25519Key(rand.Reader) // dummy shared key
			ownerBytes, _ := sharedKey.GetPublic().Marshall()

			root := &aclrecordproto.AclRoot{
				OneToOneInfo: &aclrecordproto.AclOneToOneInfo{
					Owner:   ownerBytes,
					Writers: [][]byte{writer1Bytes, {1, 2, 3}}, // invalid writer key
				},
			}

			myKey, _, _ := crypto.GenerateEd25519Key(rand.Reader)
			st := newTestAclStateWithKey(myKey)
			err := st.setOneToOneAcl("rootId", root)

			require.Error(t, err)
			assert.False(t, st.IsOneToOne())
		})
	})
	// 1

	// 1
	t.Run("deriveOneToOneKeys", func(t *testing.T) {
		t.Run("success when my key is first writer", func(t *testing.T) {
			myKey, _, _ := crypto.GenerateEd25519Key(rand.Reader)
			bobKey, _, _ := crypto.GenerateEd25519Key(rand.Reader)

			myPubKeyBytes, _ := myKey.GetPublic().Marshall()
			bobPubKeyBytes, _ := bobKey.GetPublic().Marshall()

			sharedKey, _ := crypto.GenerateSharedKey(myKey, bobKey.GetPublic(), crypto.AnysyncOneToOneSpacePath)
			ownerBytes, _ := sharedKey.GetPublic().Marshall()

			root := &aclrecordproto.AclRoot{
				OneToOneInfo: &aclrecordproto.AclOneToOneInfo{
					Owner:   ownerBytes,
					Writers: [][]byte{myPubKeyBytes, bobPubKeyBytes},
				},
			}

			st := newTestAclStateWithKey(myKey)
			err := st.deriveOneToOneKeys("rootId", root)

			require.NoError(t, err)
			assert.Contains(t, st.keys, "rootId")
			assert.NotNil(t, st.keys["rootId"].ReadKey)
			assert.NotNil(t, st.keys["rootId"].MetadataPrivKey)
			assert.NotNil(t, st.keys["rootId"].MetadataPubKey)
		})

		t.Run("success when my key is second writer", func(t *testing.T) {
			myKey, _, _ := crypto.GenerateEd25519Key(rand.Reader)
			bobKey, _, _ := crypto.GenerateEd25519Key(rand.Reader)

			myPubKeyBytes, _ := myKey.GetPublic().Marshall()
			bobPubKeyBytes, _ := bobKey.GetPublic().Marshall()

			sharedKey, _ := crypto.GenerateSharedKey(myKey, bobKey.GetPublic(), crypto.AnysyncOneToOneSpacePath)
			ownerBytes, _ := sharedKey.GetPublic().Marshall()

			root := &aclrecordproto.AclRoot{
				OneToOneInfo: &aclrecordproto.AclOneToOneInfo{
					Owner:   ownerBytes,
					Writers: [][]byte{bobPubKeyBytes, myPubKeyBytes},
				},
			}

			st := newTestAclStateWithKey(myKey)
			err := st.deriveOneToOneKeys("rootId", root)

			require.NoError(t, err)
			assert.Contains(t, st.keys, "rootId")
			assert.NotNil(t, st.keys["rootId"].ReadKey)
			assert.NotNil(t, st.keys["rootId"].MetadataPrivKey)
			assert.NotNil(t, st.keys["rootId"].MetadataPubKey)
		})

		t.Run("error when bob public key is invalid", func(t *testing.T) {
			myKey, _, _ := crypto.GenerateEd25519Key(rand.Reader)
			myPubKeyBytes, _ := myKey.GetPublic().Marshall()

			root := &aclrecordproto.AclRoot{
				OneToOneInfo: &aclrecordproto.AclOneToOneInfo{
					Owner:   []byte{1, 2, 3},
					Writers: [][]byte{myPubKeyBytes, {1, 2, 3}}, // invalid bob key
				},
			}

			st := newTestAclStateWithKey(myKey)
			err := st.deriveOneToOneKeys("rootId", root)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "error Unmarshal(bobPubKeyBytes)")
		})

		t.Run("error when owner pubkey doesn't match derived pubkey", func(t *testing.T) {
			myKey, _, _ := crypto.GenerateEd25519Key(rand.Reader)
			bobKey, _, _ := crypto.GenerateEd25519Key(rand.Reader)

			myPubKeyBytes, _ := myKey.GetPublic().Marshall()
			bobPubKeyBytes, _ := bobKey.GetPublic().Marshall()

			root := &aclrecordproto.AclRoot{
				OneToOneInfo: &aclrecordproto.AclOneToOneInfo{
					Owner:   []byte{1, 2, 3}, // wrong owner
					Writers: [][]byte{myPubKeyBytes, bobPubKeyBytes},
				},
			}

			st := newTestAclStateWithKey(myKey)
			err := st.deriveOneToOneKeys("rootId", root)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "Owner pubkey != derived pubkey")
		})
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

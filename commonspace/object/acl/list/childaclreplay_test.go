package list

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
)

// A log that holds records written on the authority of the parent builds to the same state from
// storage, without an authorizer, under every verifier. A list ingests it record by record unless it
// validates in full and does not vouch for the accepted log.
func TestChildAclReplay(t *testing.T) {
	c := newChildAcl(t, map[string]AclPermissions{
		"admin":  AclPermissionsAdmin,
		"writer": AclPermissionsWriter,
		"reader": AclPermissionsReader,
	})
	c.changeOwnerByParent(t, "admin", AclPermissionsWriter)
	c.removeNoRotate(t, "reader")
	rotation := c.rotate(t, "admin")
	// the previous owner, a Writer now, is removed in turn
	c.removeNoRotate(t, "owner")
	head := c.head()

	check := func(t *testing.T, viewer string, st *AclState) {
		t.Helper()
		require.Equal(t, head, st.LastRecordId())
		c.requireOwner(t, st, "admin")
		require.True(t, st.Permissions(c.pub("owner")).NoPermissions())
		require.True(t, st.Permissions(c.pub("reader")).NoPermissions())
		require.Equal(t, AclPermissionsWriter, st.Permissions(c.pub("writer")))
		require.True(t, st.Permissions(c.pub("parent")).NoPermissions())
		c.requireNoAccount(t, st, "parent")
		require.True(t, st.RotationPending())
		require.Equal(t, rotation, st.CurrentReadKeyId())
		holdsKey := viewer == "admin" || viewer == "writer" || viewer == "owner"
		require.Equal(t, holdsKey, st.Keys()[rotation].ReadKey != nil)
	}

	verifiers := map[string]func() recordverifier.AcceptorVerifier{
		"validating":     recordverifier.NewValidateFull,
		"accepted log":   recordverifier.NewValidateAcceptedLog,
		"network":        c.networkVerifier,
		"non-validating": func() recordverifier.AcceptorVerifier { return noValidateVerifier{} },
	}
	for _, viewer := range []string{"admin", "writer", "owner", "reader", "parent", "coordinator"} {
		reference := c.build(t, viewer, recordverifier.NewValidateFull()).AclState()
		check(t, viewer, reference)

		for name, verifier := range verifiers {
			t.Run(viewer+"/"+name, func(t *testing.T) {
				built := c.build(t, viewer, verifier())
				check(t, viewer, built.AclState())
				requireSameAclState(t, reference, built.AclState())

				// ingested record by record into a store, then built from that store
				ctx := context.Background()
				store := createStore(ctx, t)
				headStorage, err := headstorage.New(ctx, store)
				require.NoError(t, err)
				storage, err := CreateStorage(ctx, c.log[0], headStorage, store)
				require.NoError(t, err)
				ingested, err := BuildAclListWithIdentity(c.keys[viewer], storage, verifier())
				require.NoError(t, err)
				err = ingested.AddRawRecords(c.log[1:])
				if name == "validating" {
					// the list stops before the first record written on the authority of the parent
					require.ErrorIs(t, err, ErrInsufficientPermissions)
					require.Equal(t, c.log[1].Id, ingested.AclState().LastRecordId())
					c.requireOwner(t, ingested.AclState(), "owner")
					ingested, err = BuildAclListWithIdentity(c.keys[viewer], storage, recordverifier.NewValidateAcceptedLog())
					require.NoError(t, err)
					err = ingested.AddRawRecords(c.log[1:])
				}
				require.NoError(t, err)
				check(t, viewer, ingested.AclState())
				requireSameAclState(t, reference, ingested.AclState())

				fromStore, err := BuildAclListWithIdentity(c.keys[viewer], storage, verifier())
				require.NoError(t, err)
				check(t, viewer, fromStore.AclState())
				requireSameAclState(t, reference, fromStore.AclState())
			})
		}
	}
}

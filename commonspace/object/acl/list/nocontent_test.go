package list

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/listtest"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

// Permission checks live in the per-type apply handlers, so a record whose content reaches none of them
// would be applied without checking its author. These records carry no applicable content and are signed
// by an identity that is not in the acl at all.
var noApplicableContent = []struct {
	name string
	data []byte
	err  error
}{
	// AclData{AclContent: [{}]}: one content value with the oneof unset
	{name: "empty content value", data: []byte{0x0a, 0x00}, err: ErrUnexpectedContentType},
	// one content value whose only field (99, length-delimited, empty) this build does not know — how a
	// content type added in a later release decodes
	{name: "unknown content type", data: []byte{0x0a, 0x03, 0x9a, 0x06, 0x00}, err: ErrUnexpectedContentType},
	{name: "no data", data: nil, err: ErrNoAclContent},
	// AclData carrying only an unknown field (99, varint): non-empty bytes, no content values
	{name: "only an unknown field", data: []byte{0x98, 0x06, 0x01}, err: ErrNoAclContent},
}

func signAclRecord(t *testing.T, keys *accountdata.AccountKeys, prevId string, data []byte) (*consensusproto.RawRecord, *consensusproto.RawRecordWithId) {
	identity, err := keys.SignKey.GetPublic().Marshall()
	require.NoError(t, err)
	payload, err := (&consensusproto.Record{
		PrevId:    prevId,
		Identity:  identity,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}).MarshalVT()
	require.NoError(t, err)
	signature, err := keys.SignKey.Sign(payload)
	require.NoError(t, err)
	raw := &consensusproto.RawRecord{Payload: payload, Signature: signature}
	return raw, listtest.WrapAclRecord(raw)
}

func TestAclList_ValidateRawRecordRejectsNoApplicableContent(t *testing.T) {
	for _, tc := range noApplicableContent {
		t.Run(tc.name, func(t *testing.T) {
			fx := newFixture(t)
			stranger, err := accountdata.NewRandom()
			require.NoError(t, err)
			head := fx.ownerAcl.AclState().LastRecordId()

			raw, _ := signAclRecord(t, stranger, head, tc.data)
			require.ErrorIs(t, fx.ownerAcl.ValidateRawRecord(raw, nil), tc.err)
			require.Equal(t, head, fx.ownerAcl.AclState().LastRecordId())
		})
	}
}

// An acl may already hold such records: the network accepted them before admission checked for them, and
// a content type added later reaches old clients the same way. Ingesting, rebuilding and migrating the log
// must not fail on them, whichever verifier the list uses — node stats, migration and some client paths
// build from storage with a validating one.
func TestAclList_StoredNoApplicableContentStillBuilds(t *testing.T) {
	verifiers := map[string]recordverifier.AcceptorVerifier{
		"validating":     recordverifier.NewValidateFull(),
		"non-validating": noValidateVerifier{},
	}
	for _, tc := range noApplicableContent {
		for vName, verifier := range verifiers {
			t.Run(tc.name+"/"+vName, func(t *testing.T) {
				fx := newFixture(t)
				stranger, err := accountdata.NewRandom()
				require.NoError(t, err)

				_, withId := signAclRecord(t, stranger, fx.ownerAcl.AclState().LastRecordId(), tc.data)
				require.NoError(t, fx.ownerAcl.AddRawRecord(withId))
				require.Equal(t, withId.Id, fx.ownerAcl.AclState().LastRecordId())

				rebuilt, err := BuildAclListWithIdentity(fx.ownerKeys, fx.ownerAcl.storage, verifier)
				require.NoError(t, err)
				require.Equal(t, withId.Id, rebuilt.AclState().LastRecordId())
				require.True(t, rebuilt.AclState().Permissions(stranger.SignKey.GetPublic()).NoPermissions())

				// the owner still builds on top of it
				_, err = rebuilt.RecordBuilder().BuildInvite()
				require.NoError(t, err)

				// and a fresh list ingests the whole log, as the storage migration does
				ctx := context.Background()
				store := createStore(ctx, t)
				headStorage, err := headstorage.New(ctx, store)
				require.NoError(t, err)
				storage, err := CreateStorage(ctx, fx.ownerAcl.Root(), headStorage, store)
				require.NoError(t, err)
				migrated, err := BuildAclListWithIdentity(fx.ownerKeys, storage, verifier)
				require.NoError(t, err)
				var records []*consensusproto.RawRecordWithId
				for _, rec := range fx.ownerAcl.Records()[1:] {
					raw, err := fx.ownerAcl.storage.Get(ctx, rec.Id)
					require.NoError(t, err)
					records = append(records, raw.RawRecordWithId())
				}
				require.NoError(t, migrated.AddRawRecords(records))
				require.Equal(t, withId.Id, migrated.AclState().LastRecordId())
			})
		}
	}
}

func TestAclRecordBuilder_EmptyBatchRequest(t *testing.T) {
	fx := newFixture(t)
	_, err := fx.ownerAcl.RecordBuilder().BuildBatchRequest(BatchRequestPayload{})
	require.ErrorIs(t, err, ErrNoAclContent)
}

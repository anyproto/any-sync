package acl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/consensus/consensusclient"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/consensus/consensusproto/consensuserr"
)

// The consensus client can deliver more than one event for a watch before the object is dropped:
// it hands a whole batch to the watcher, and a watch restored after a reconnect can report the same
// missing log twice. Only the first event decides how the object loads.
func TestAclObject_EventsAfterTheFirst(t *testing.T) {
	ownerKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	const spaceId = "spaceId"
	ownerAcl, err := list.NewInMemoryDerivedAcl(spaceId, ownerKeys)
	require.NoError(t, err)
	root := func() []*consensusproto.RawRecordWithId {
		return []*consensusproto.RawRecordWithId{ownerAcl.Root()}
	}

	// load creates the object while the watch delivers events, and waits until all of them are handled
	load := func(t *testing.T, events ...func(w consensusclient.Watcher)) (*aclObject, error) {
		fx := newFixture(t)
		defer fx.finish(t)
		handled := make(chan struct{})
		fx.consCl.EXPECT().Watch(spaceId, gomock.Any()).DoAndReturn(func(_ string, w consensusclient.Watcher) error {
			go func() {
				defer close(handled)
				for _, event := range events {
					event(w)
				}
			}()
			return nil
		})
		fx.consCl.EXPECT().UnWatch(spaceId).AnyTimes()
		obj, err := fx.AclService.(*aclService).newAclObject(ctx, spaceId)
		<-handled
		return obj, err
	}
	notFound := func(w consensusclient.Watcher) { w.AddConsensusError(consensuserr.ErrLogNotFound) }
	records := func(w consensusclient.Watcher) { w.AddConsensusRecords(root()) }

	t.Run("records after an error are dropped", func(t *testing.T) {
		_, err := load(t, notFound, records)
		assert.ErrorIs(t, err, consensuserr.ErrLogNotFound)
	})
	t.Run("a second error is dropped", func(t *testing.T) {
		_, err := load(t, notFound, notFound)
		assert.ErrorIs(t, err, consensuserr.ErrLogNotFound)
	})
	t.Run("records after records that do not build a list are dropped", func(t *testing.T) {
		broken := func(w consensusclient.Watcher) {
			w.AddConsensusRecords([]*consensusproto.RawRecordWithId{{Id: "broken", Payload: []byte("broken")}})
		}
		_, err := load(t, broken, records)
		assert.Error(t, err)
	})
	t.Run("an error after the records leaves the object loaded", func(t *testing.T) {
		obj, err := load(t, records, notFound)
		require.NoError(t, err)
		assert.Equal(t, ownerAcl.Id(), obj.Id())
	})
}

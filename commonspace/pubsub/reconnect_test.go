package pubsub

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/pubsub/pubsubproto"
)

// rawStream opens a direct PubSubStream from client fx to server target, bypassing
// the client engine's pool so a test can drive two independent streams from the
// same peerId (as a reconnect produces server-side).
func rawStream(t *testing.T, client, target *engineFx) pubsubproto.DRPCPubSub_PubSubStreamClient {
	p := connect(t, client, target)
	conn, err := p.AcquireDrpcConn(testCtx)
	require.NoError(t, err)
	stream, err := pubsubproto.NewDRPCPubSubClient(conn).PubSubStream(p.Context())
	require.NoError(t, err)
	return stream
}

func remoteMatchCount(fx *engineFx, spaceId, topic string) int {
	fx.svc.remoteMu.Lock()
	defer fx.svc.remoteMu.Unlock()
	si := fx.svc.remote[spaceId]
	if si == nil {
		return 0
	}
	return len(si.trie.Match(topic, nil))
}

func waitMatchCount(t *testing.T, fx *engineFx, spaceId, topic string, want int) {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if remoteMatchCount(fx, spaceId, topic) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("match count for %s never reached %d (got %d)", topic, want, remoteMatchCount(fx, spaceId, topic))
}

// TestReconnectKeepsInterest is the regression test for the streamId-keyed interest
// fix: two streams from the SAME peer each hold interest, and closing one must not
// wipe the other's — a stale reconnecting stream must not take the fresh one down.
func TestReconnectKeepsInterest(t *testing.T) {
	membership := &fakeMembership{}
	relay := &fakeRelay{}
	node := newEngineFx(t, "node", membership, relay)
	defer node.finish()
	client := newEngineFx(t, "client", membership, nil)
	defer client.finish()
	membership.allow(client.identity())

	subMsg := &pubsubproto.PubSubMessage{Content: &pubsubproto.PubSubMessage_Subscribe{
		Subscribe: &pubsubproto.Subscribe{SpaceId: testSpace, Topics: []string{"chat/>"}},
	}}

	// stream 1 subscribes
	s1 := rawStream(t, client, node)
	require.NoError(t, s1.Send(subMsg))
	waitMatchCount(t, node, testSpace, "chat/x", 1)

	// stream 2 (same peerId) subscribes to the same pattern — the node must track
	// it independently, so the trie refcount is now 2 even though Match still
	// returns the single pattern
	s2 := rawStream(t, client, node)
	require.NoError(t, s2.Send(subMsg))
	// give the node time to process s2's subscribe
	time.Sleep(200 * time.Millisecond)
	require.Equal(t, 1, remoteMatchCount(node, testSpace, "chat/x"))

	// close stream 1 — the stale stream must NOT take down stream 2's interest
	require.NoError(t, s1.Close())
	// interest must remain after s1's close is processed
	time.Sleep(300 * time.Millisecond)
	require.Equal(t, 1, remoteMatchCount(node, testSpace, "chat/x"),
		"closing the stale stream wiped the fresh stream's interest")

	// closing stream 2 finally clears it
	require.NoError(t, s2.Close())
	waitMatchCount(t, node, testSpace, "chat/x", 0)

	// and the per-stream bookkeeping is fully drained
	node.svc.remoteMu.Lock()
	remainingStreams := len(node.svc.streams)
	remainingSpaces := len(node.svc.remote)
	node.svc.remoteMu.Unlock()
	require.Equal(t, 0, remainingStreams, "stream records leaked")
	require.Equal(t, 0, remainingSpaces, "space records leaked")
}

// TestStreamCloseDrainsInterest verifies a single stream's interest and all
// bookkeeping is removed when it closes without an explicit unsubscribe.
func TestStreamCloseDrainsInterest(t *testing.T) {
	membership := &fakeMembership{}
	relay := &fakeRelay{}
	node := newEngineFx(t, "node", membership, relay)
	defer node.finish()
	client := newEngineFx(t, "client", membership, nil)
	defer client.finish()
	membership.allow(client.identity())

	s := rawStream(t, client, node)
	require.NoError(t, s.Send(&pubsubproto.PubSubMessage{Content: &pubsubproto.PubSubMessage_Subscribe{
		Subscribe: &pubsubproto.Subscribe{SpaceId: testSpace, Topics: []string{"a/*", "b/>", "c"}},
	}}))
	waitMatchCount(t, node, testSpace, "a/x", 1)

	require.NoError(t, s.Close())
	waitMatchCount(t, node, testSpace, "a/x", 0)
	node.svc.remoteMu.Lock()
	defer node.svc.remoteMu.Unlock()
	require.Empty(t, node.svc.streams)
	require.Empty(t, node.svc.remote)
}

// TestCapExceededReportsOnlyRejected verifies that when a subscribe exceeds the
// per-space pattern cap mid-batch, the TooManyTopics status echoes only the
// rejected patterns, and the ones accepted before the cap stay live.
func TestCapExceededReportsOnlyRejected(t *testing.T) {
	membership := &fakeMembership{}
	relay := &fakeRelay{}
	node := newEngineFx(t, "node", membership, relay)
	defer node.finish()
	// tiny per-space cap so a 3-topic subscribe trips it after 2
	node.svc.cfg.MaxPatternsPerSpace = 2
	client := newEngineFx(t, "client", membership, nil)
	defer client.finish()
	membership.allow(client.identity())

	s := rawStream(t, client, node)
	require.NoError(t, s.Send(&pubsubproto.PubSubMessage{Content: &pubsubproto.PubSubMessage_Subscribe{
		Subscribe: &pubsubproto.Subscribe{SpaceId: testSpace, Topics: []string{"a", "b", "c"}},
	}}))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := s.Recv()
		require.NoError(t, err)
		if st := msg.GetStatus(); st != nil {
			require.Equal(t, pubsubproto.ErrCodes_TooManyTopics, st.Code)
			require.Equal(t, []string{"c"}, st.Topics, "only the rejected topic is reported")
			// the two accepted before the cap are live
			require.Equal(t, 1, remoteMatchCount(node, testSpace, "a"))
			require.Equal(t, 1, remoteMatchCount(node, testSpace, "b"))
			return
		}
	}
	t.Fatal("no TooManyTopics status received")
}

// TestInvalidSpaceIdRejected ensures a spaceId containing '/' (which would break
// tag parsing) is rejected at subscribe with InvalidTopic.
func TestInvalidSpaceIdRejected(t *testing.T) {
	membership := &fakeMembership{}
	node := newEngineFx(t, "node", membership, &fakeRelay{})
	defer node.finish()
	client := newEngineFx(t, "client", membership, nil)
	defer client.finish()
	membership.allow(client.identity())

	s := rawStream(t, client, node)
	require.NoError(t, s.Send(&pubsubproto.PubSubMessage{Content: &pubsubproto.PubSubMessage_Subscribe{
		Subscribe: &pubsubproto.Subscribe{SpaceId: "bad/space", Topics: []string{"chat/>"}},
	}}))
	// the node replies with a Status frame we can read directly off the raw stream
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := s.Recv()
		require.NoError(t, err)
		if st := msg.GetStatus(); st != nil {
			require.Equal(t, pubsubproto.ErrCodes_InvalidTopic, st.Code)
			return
		}
	}
	t.Fatal("no InvalidTopic status received")
}

// TestEvictMemberStopsDelivery verifies §6.4 active eviction: after EvictMember,
// a removed member's stream stops receiving even though its stream stays open, and
// another member subscribed to the same pattern keeps receiving.
func TestEvictMemberStopsDelivery(t *testing.T) {
	n := newNetFx(t)
	defer n.finish()

	_, err := n.clientA.svc.Subscribe(testSpace, "chat/>", n.clientA.handler())
	require.NoError(t, err)
	_, err = n.clientC.svc.Subscribe(testSpace, "chat/>", n.clientC.handler())
	require.NoError(t, err)
	waitMatchCount(t, n.nodeB, testSpace, "chat/x", 1)
	// wait until both members' interest is registered on the node
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		n.nodeB.svc.remoteMu.Lock()
		nstreams := len(n.nodeB.svc.streams)
		n.nodeB.svc.remoteMu.Unlock()
		if nstreams == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// evict clientA at the node
	n.nodeB.svc.EvictMember(testSpace, n.clientA.identity())

	// clientC (still a member) publishes; clientC receives, clientA does not
	require.NoError(t, n.clientC.svc.Publish(testCtx, testSpace, "chat/x", []byte("members-only")))
	require.Equal(t, "members-only", waitReceived(t, n.clientC).payload)
	expectSilence(t, n.clientA, 400*time.Millisecond)
}

// TestRevalidateMembersEvictsNonMembers verifies the node-side ACL-change hook:
// RevalidateMembers evicts subscribers whose account no longer passes the member
// predicate while keeping current members subscribed.
func TestRevalidateMembersEvictsNonMembers(t *testing.T) {
	n := newNetFx(t)
	defer n.finish()

	_, err := n.clientA.svc.Subscribe(testSpace, "chat/>", n.clientA.handler())
	require.NoError(t, err)
	_, err = n.clientC.svc.Subscribe(testSpace, "chat/>", n.clientC.handler())
	require.NoError(t, err)
	waitMatchCount(t, n.nodeB, testSpace, "chat/x", 1)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		n.nodeB.svc.remoteMu.Lock()
		nstreams := len(n.nodeB.svc.streams)
		n.nodeB.svc.remoteMu.Unlock()
		if nstreams == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// only clientC remains a member; clientA is evicted in one pass
	keep := n.clientC.identity().Account()
	n.nodeB.svc.RevalidateMembers(testSpace, func(account string) bool {
		return account == keep
	})

	require.NoError(t, n.clientC.svc.Publish(testCtx, testSpace, "chat/x", []byte("survivors")))
	require.Equal(t, "survivors", waitReceived(t, n.clientC).payload)
	expectSilence(t, n.clientA, 400*time.Millisecond)
}

// TestCloseSpaceDropsInterest verifies CloseSpace tears down both local and
// serving-side interest so a global service retains nothing for a closed space.
func TestCloseSpaceDropsInterest(t *testing.T) {
	n := newNetFx(t)
	defer n.finish()

	_, err := n.clientA.svc.Subscribe(testSpace, "chat/>", n.clientA.handler())
	require.NoError(t, err)
	waitMatchCount(t, n.nodeB, testSpace, "chat/x", 1)

	// node unloads the space
	n.nodeB.svc.CloseSpace(testSpace)
	waitMatchCount(t, n.nodeB, testSpace, "chat/x", 0)

	// client closes the space: local interest is gone
	n.clientA.svc.CloseSpace(testSpace)
	n.clientA.svc.localMu.Lock()
	_, hasTrie := n.clientA.svc.localTrie[testSpace]
	n.clientA.svc.localMu.Unlock()
	require.False(t, hasTrie, "local interest retained after CloseSpace")
}

// TestResyncRestoresDeliveryAfterDrop verifies the reconnect watcher: after a
// subscriber's stream drops and a fresh peer replaces it, the periodic resync
// re-pushes interest and delivery resumes without the app re-subscribing.
func TestResyncRestoresDeliveryAfterDrop(t *testing.T) {
	membership := &fakeMembership{}
	relay := &fakeRelay{}
	node := newEngineFx(t, "node", membership, relay)
	defer node.finish()
	sub := newEngineFx(t, "sub", membership, nil)
	defer sub.finish()
	pub := newEngineFx(t, "pub", membership, nil)
	defer pub.finish()
	membership.allow(sub.identity(), pub.identity())

	sub.peers.add(connect(t, sub, node))
	pub.peers.add(connect(t, pub, node))

	_, err := sub.svc.Subscribe(testSpace, "chat/>", sub.handler())
	require.NoError(t, err)
	waitInterest(t, node, "chat/x")

	require.NoError(t, pub.svc.Publish(testCtx, testSpace, "chat/x", []byte("before")))
	require.Equal(t, "before", waitReceived(t, sub).payload)

	// drop the subscriber's connection and replace it with a fresh peer, as a real
	// reconnect would; the node sees the stale stream close and (eventually) a new one
	for _, p := range sub.ownedPeers {
		_ = p.Close()
	}
	sub.peers.replace(connect(t, sub, node))

	// the periodic resync re-pushes interest over the new peer; publish is
	// fire-and-forget so retry until a message lands (delivery resumes once resync
	// has reopened the stream and re-registered interest on the node)
	deadline := time.Now().Add(3 * time.Second)
	for {
		require.NoError(t, pub.svc.Publish(testCtx, testSpace, "chat/x", []byte("after")))
		select {
		case r := <-sub.received:
			require.Equal(t, "after", r.payload)
			return
		case <-time.After(200 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatal("delivery did not resume after reconnect")
		}
	}
}

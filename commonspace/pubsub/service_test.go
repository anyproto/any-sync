package pubsub

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/pubsub/pubsubproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/any-sync/util/crypto"
)

var testCtx = context.Background()

const testSpace = "space1"

//
// fakes
//

type fakeMembership struct {
	mu      sync.Mutex
	allowed map[string]bool // account id -> member
}

func (f *fakeMembership) allow(accounts ...crypto.PubKey) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.allowed == nil {
		f.allowed = make(map[string]bool)
	}
	for _, a := range accounts {
		f.allowed[a.Account()] = true
	}
}

func (f *fakeMembership) CheckMember(_ context.Context, _ string, identity crypto.PubKey) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.allowed[identity.Account()] {
		return nil
	}
	return fmt.Errorf("not a member")
}

type staticPeers struct {
	mu    sync.Mutex
	peers []peer.Peer
}

func (s *staticPeers) add(p peer.Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peers = append(s.peers, p)
}

func (s *staticPeers) replace(p peer.Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peers = []peer.Peer{p}
}

func (s *staticPeers) SpacePeers(_ context.Context, _ string) ([]peer.Peer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]peer.Peer(nil), s.peers...), nil
}

type fakeRelay struct {
	mu           sync.Mutex
	nodePeerIds  map[string]bool
	others       []peer.Peer
	forwardCalls atomic.Int32
}

func (f *fakeRelay) addNodePeer(peerId string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.nodePeerIds == nil {
		f.nodePeerIds = make(map[string]bool)
	}
	f.nodePeerIds[peerId] = true
}

func (f *fakeRelay) addOther(p peer.Peer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.others = append(f.others, p)
}

func (f *fakeRelay) IsResponsible(string) bool { return true }

func (f *fakeRelay) IsResponsibleNode(_, peerId string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.nodePeerIds[peerId]
}

func (f *fakeRelay) OtherResponsiblePeers(_ context.Context, _ string) ([]peer.Peer, error) {
	f.forwardCalls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]peer.Peer(nil), f.others...), nil
}

//
// fixture
//

type received struct {
	topic   string
	account string
	payload string
}

type engineFx struct {
	t          *testing.T
	name       string
	acc        *accountdata.AccountKeys
	svc        *service
	app        *app.App
	ts         *rpctest.TestServer
	membership *fakeMembership
	peers      *staticPeers
	relay      *fakeRelay
	statuses   chan *pubsubproto.Status
	received   chan received
	ownedPeers []peer.Peer
}

func (fx *engineFx) identity() crypto.PubKey { return fx.acc.SignKey.GetPublic() }

func (fx *engineFx) finish() {
	require.NoError(fx.t, fx.app.Close(testCtx))
	for _, p := range fx.ownedPeers {
		_ = p.Close()
	}
}

func newEngineFx(t *testing.T, name string, membership *fakeMembership, relay *fakeRelay) *engineFx {
	acc, err := accountdata.NewRandom()
	require.NoError(t, err)
	fx := &engineFx{
		t:          t,
		name:       name,
		acc:        acc,
		membership: membership,
		relay:      relay,
		statuses:   make(chan *pubsubproto.Status, 16),
		received:   make(chan received, 64),
	}
	deps := Deps{
		Membership: membership,
		OnStatus:   func(_ string, st *pubsubproto.Status) { fx.statuses <- st },
		// fast resync so reconnect tests don't wait the 20s default
		Config: Config{ResyncInterval: 150 * time.Millisecond},
	}
	if relay != nil {
		deps.Relay = relay
	} else {
		fx.peers = &staticPeers{}
		deps.Peers = fx.peers
	}
	fx.svc = New(deps).(*service)

	fx.app = new(app.App)
	fx.app.Register(accounttest.NewWithAcc(acc)).Register(fx.svc)
	require.NoError(t, fx.app.Start(testCtx))

	fx.ts = rpctest.NewTestServer()
	require.NoError(t, RegisterRpc(fx.ts.Mux, fx.svc))
	return fx
}

func (fx *engineFx) handler() Handler {
	return func(_, topic string, identity crypto.PubKey, payload []byte) {
		fx.received <- received{topic: topic, account: identity.Account(), payload: string(payload)}
	}
}

// connect wires from -> to and returns from's peer handle for to.
// Both directions carry the respective remote's peerId and identity in ctx,
// mirroring what secureservice puts there in production.
func connect(t *testing.T, from, to *engineFx) peer.Peer {
	fromIdentity, err := from.identity().Marshall()
	require.NoError(t, err)
	toIdentity, err := to.identity().Marshall()
	require.NoError(t, err)
	mcAtTo, mcAtFrom := rpctest.MultiConnPairWithClientServerIdentity(
		from.acc.PeerId, to.acc.PeerId, fromIdentity, toIdentity)
	pAtTo, err := peer.NewPeer(mcAtTo, to.ts)
	require.NoError(t, err)
	to.ownedPeers = append(to.ownedPeers, pAtTo)
	pAtFrom, err := peer.NewPeer(mcAtFrom, from.ts)
	require.NoError(t, err)
	from.ownedPeers = append(from.ownedPeers, pAtFrom)
	return pAtFrom
}

// waitInterest polls the serving engine until topic matches remote interest,
// removing the subscribe/publish race inherent to fire-and-forget semantics.
func waitInterest(t *testing.T, serving *engineFx, topic string) {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		serving.svc.remoteMu.Lock()
		si := serving.svc.remote[testSpace]
		var n int
		if si != nil {
			n = len(si.trie.Match(topic, nil))
		}
		serving.svc.remoteMu.Unlock()
		if n > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("interest for %s never registered on %s", topic, serving.name)
}

func waitReceived(t *testing.T, fx *engineFx) received {
	select {
	case r := <-fx.received:
		return r
	case <-time.After(2 * time.Second):
		t.Fatalf("%s: timeout waiting for message", fx.name)
		return received{}
	}
}

func expectSilence(t *testing.T, fx *engineFx, d time.Duration) {
	select {
	case r := <-fx.received:
		t.Fatalf("%s: unexpected message %+v", fx.name, r)
	case <-time.After(d):
	}
}

func waitStatus(t *testing.T, fx *engineFx, code pubsubproto.ErrCodes) {
	deadline := time.After(2 * time.Second)
	for {
		select {
		case st := <-fx.statuses:
			if st.Code == code {
				return
			}
		case <-deadline:
			t.Fatalf("%s: timeout waiting for status %s", fx.name, code)
		}
	}
}

//
// tests
//

// topology: clientA, clientC -> nodeB <-> nodeB2 <- clientA2
type netFx struct {
	nodeB, nodeB2, clientA, clientC, clientA2 *engineFx
}

func newNetFx(t *testing.T) *netFx {
	membership := &fakeMembership{}
	relayB := &fakeRelay{}
	relayB2 := &fakeRelay{}

	n := &netFx{
		nodeB:    newEngineFx(t, "nodeB", membership, relayB),
		nodeB2:   newEngineFx(t, "nodeB2", membership, relayB2),
		clientA:  newEngineFx(t, "clientA", membership, nil),
		clientC:  newEngineFx(t, "clientC", membership, nil),
		clientA2: newEngineFx(t, "clientA2", membership, nil),
	}
	membership.allow(n.clientA.identity(), n.clientC.identity(), n.clientA2.identity())

	n.clientA.peers.add(connect(t, n.clientA, n.nodeB))
	n.clientC.peers.add(connect(t, n.clientC, n.nodeB))
	n.clientA2.peers.add(connect(t, n.clientA2, n.nodeB2))
	relayB.addOther(connect(t, n.nodeB, n.nodeB2))
	relayB2.addNodePeer(n.nodeB.acc.PeerId)
	relayB.addNodePeer(n.nodeB2.acc.PeerId)
	return n
}

func (n *netFx) finish() {
	for _, fx := range []*engineFx{n.clientA, n.clientC, n.clientA2, n.nodeB, n.nodeB2} {
		fx.finish()
	}
}

func TestPubSubFanoutAndWildcards(t *testing.T) {
	n := newNetFx(t)
	defer n.finish()

	_, err := n.clientA.svc.Subscribe(testSpace, "chat/>", n.clientA.handler())
	require.NoError(t, err)
	_, err = n.clientA.svc.Subscribe(testSpace, "acc/online/*", n.clientA.handler())
	require.NoError(t, err)
	waitInterest(t, n.nodeB, "chat/room1/typing")
	waitInterest(t, n.nodeB, "acc/online/"+n.clientC.identity().Account())

	require.NoError(t, n.clientC.svc.Publish(testCtx, testSpace, "chat/room1/typing", []byte("tick")))
	r := waitReceived(t, n.clientA)
	require.Equal(t, "chat/room1/typing", r.topic)
	require.Equal(t, n.clientC.identity().Account(), r.account)
	require.Equal(t, "tick", r.payload)

	ownTopic := "acc/online/" + n.clientC.identity().Account()
	require.NoError(t, n.clientC.svc.Publish(testCtx, testSpace, ownTopic, []byte("on")))
	r = waitReceived(t, n.clientA)
	require.Equal(t, ownTopic, r.topic)

	// no matching interest: fire-and-forget discards silently
	require.NoError(t, n.clientC.svc.Publish(testCtx, testSpace, "other/topic", []byte("x")))
	expectSilence(t, n.clientA, 300*time.Millisecond)
}

func TestPubSubNodeRelay(t *testing.T) {
	n := newNetFx(t)
	defer n.finish()

	_, err := n.clientA2.svc.Subscribe(testSpace, "chat/>", n.clientA2.handler())
	require.NoError(t, err)
	waitInterest(t, n.nodeB2, "chat/x")

	require.NoError(t, n.clientC.svc.Publish(testCtx, testSpace, "chat/x", []byte("cross-node")))
	r := waitReceived(t, n.clientA2)
	require.Equal(t, "chat/x", r.topic)
	require.Equal(t, "cross-node", r.payload)

	// hop limit: B2 must not re-forward the relayed message
	require.Equal(t, int32(0), n.nodeB2.relay.forwardCalls.Load())
	// B forwarded exactly once
	require.Equal(t, int32(1), n.nodeB.relay.forwardCalls.Load())
}

func TestPubSubMembershipRejection(t *testing.T) {
	n := newNetFx(t)
	defer n.finish()

	outsider := newEngineFx(t, "outsider", n.nodeB.membership, nil)
	defer outsider.finish()
	outsider.peers.add(connect(t, outsider, n.nodeB))

	_, err := outsider.svc.Subscribe(testSpace, "chat/>", outsider.handler())
	require.NoError(t, err) // local registration succeeds, the node rejects async
	waitStatus(t, outsider, pubsubproto.ErrCodes_NotAMember)

	require.NoError(t, outsider.svc.Publish(testCtx, testSpace, "chat/x", []byte("spam")))
	waitStatus(t, outsider, pubsubproto.ErrCodes_NotAMember)
}

func TestPubSubTopicOwnership(t *testing.T) {
	n := newNetFx(t)
	defer n.finish()

	victimTopic := "acc/online/" + n.clientA.identity().Account()
	_, err := n.clientA.svc.Subscribe(testSpace, "acc/online/*", n.clientA.handler())
	require.NoError(t, err)
	waitInterest(t, n.nodeB, victimTopic)

	// the client fails fast when publishing into someone else's self-owned topic
	require.ErrorIs(t, n.clientC.svc.Publish(testCtx, testSpace, victimTopic, []byte("spoof")),
		pubsubproto.ErrTopicNotOwned)

	// relay-side enforcement: a malicious client bypassing the local check is
	// rejected at the node ingress and never fanned out
	spoofed := &pubsubproto.Publish{
		SpaceId:        testSpace,
		Topic:          victimTopic,
		MsgId:          testMsgId(776),
		Payload:        []byte("spoof"),
		TimestampMilli: time.Now().UnixMilli(),
	}
	require.NoError(t, signPublish(n.clientC.acc.SignKey, spoofed))
	spoofCtx := peer.CtxWithPeerId(peer.CtxWithIdentity(testCtx, spoofed.Identity), n.clientC.acc.PeerId)
	n.nodeB.svc.handlePublish(spoofCtx, n.clientC.acc.PeerId, spoofed)
	expectSilence(t, n.clientA, 300*time.Millisecond)

	// receive-side enforcement: a forged message injected past the relay is dropped
	forged := &pubsubproto.Publish{
		SpaceId:        testSpace,
		Topic:          victimTopic,
		MsgId:          testMsgId(777),
		Payload:        []byte("forged"),
		TimestampMilli: time.Now().UnixMilli(),
	}
	require.NoError(t, signPublish(n.clientC.acc.SignKey, forged))
	n.clientA.svc.receivePublish(testCtx, forged)
	expectSilence(t, n.clientA, 300*time.Millisecond)
}

func TestPubSubEchoSuppression(t *testing.T) {
	n := newNetFx(t)
	defer n.finish()

	_, err := n.clientA.svc.Subscribe(testSpace, "chat/>", n.clientA.handler())
	require.NoError(t, err)
	waitInterest(t, n.nodeB, "chat/self")

	require.NoError(t, n.clientA.svc.Publish(testCtx, testSpace, "chat/self", []byte("echo?")))
	r := waitReceived(t, n.clientA)
	require.Equal(t, "echo?", r.payload)
	// the node echoes the message back to A's subscribed stream; dedup must drop it
	expectSilence(t, n.clientA, 500*time.Millisecond)
}

func TestPubSubStaleMessageDropped(t *testing.T) {
	n := newNetFx(t)
	defer n.finish()

	_, err := n.clientA.svc.Subscribe(testSpace, "chat/>", n.clientA.handler())
	require.NoError(t, err)

	// a validly-signed message with a timestamp far in the past is dropped even
	// though its signature verifies — raising the replay bar after dedup eviction
	stale := &pubsubproto.Publish{
		SpaceId:        testSpace,
		Topic:          "chat/old",
		MsgId:          testMsgId(999),
		Payload:        []byte("replayed"),
		TimestampMilli: time.Now().Add(-time.Hour).UnixMilli(),
	}
	require.NoError(t, signPublish(n.clientC.acc.SignKey, stale))
	n.clientA.svc.receivePublish(testCtx, stale)
	expectSilence(t, n.clientA, 300*time.Millisecond)

	// a fresh message from the same sender is delivered
	fresh := &pubsubproto.Publish{
		SpaceId:        testSpace,
		Topic:          "chat/new",
		MsgId:          testMsgId(1000),
		Payload:        []byte("fresh"),
		TimestampMilli: time.Now().UnixMilli(),
	}
	require.NoError(t, signPublish(n.clientC.acc.SignKey, fresh))
	n.clientA.svc.receivePublish(testCtx, fresh)
	require.Equal(t, "fresh", waitReceived(t, n.clientA).payload)
}

func TestPubSubDuplicatePathSuppression(t *testing.T) {
	n := newNetFx(t)
	defer n.finish()

	_, err := n.clientA.svc.Subscribe(testSpace, "chat/>", n.clientA.handler())
	require.NoError(t, err)

	p := &pubsubproto.Publish{
		SpaceId:        testSpace,
		Topic:          "chat/dup",
		MsgId:          testMsgId(555),
		Payload:        []byte("once"),
		TimestampMilli: time.Now().UnixMilli(),
	}
	require.NoError(t, signPublish(n.clientC.acc.SignKey, p))
	// the same message arrives twice (LAN path + node path)
	n.clientA.svc.receivePublish(testCtx, p)
	n.clientA.svc.receivePublish(testCtx, p)
	r := waitReceived(t, n.clientA)
	require.Equal(t, "once", r.payload)
	expectSilence(t, n.clientA, 300*time.Millisecond)
}

func TestPubSubUnsubscribe(t *testing.T) {
	n := newNetFx(t)
	defer n.finish()

	unsub, err := n.clientA.svc.Subscribe(testSpace, "chat/>", n.clientA.handler())
	require.NoError(t, err)
	waitInterest(t, n.nodeB, "chat/x")

	unsub()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		n.nodeB.svc.remoteMu.Lock()
		si := n.nodeB.svc.remote[testSpace]
		n.nodeB.svc.remoteMu.Unlock()
		if si == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.NoError(t, n.clientC.svc.Publish(testCtx, testSpace, "chat/x", []byte("gone")))
	expectSilence(t, n.clientA, 300*time.Millisecond)
}

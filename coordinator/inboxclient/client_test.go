package inboxclient

import (
	"context"
	"crypto/rand"
	"sync"
	"testing"
	"time"

	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ctx = context.Background()

func TestInbox_Fetch(t *testing.T) {
	var makeClientServer = func(t *testing.T, ts *testServer) (fxC, fxS *fixture, peerId string) {
		fxC = newFixture(t, nil)
		fxS = newFixture(t, ts)
		peerId = "peer"
		identity, err := fxC.account.Account().SignKey.GetPublic().Marshall()
		require.NoError(t, err)
		mcS, mcC := rpctest.MultiConnPairWithIdentity(peerId, peerId+"client", identity)
		pS, err := peer.NewPeer(mcS, fxC.ts)
		require.NoError(t, err)
		fxC.tp.AddPeer(ctx, pS)
		_, err = peer.NewPeer(mcC, fxS.ts)
		require.NoError(t, err)
		return
	}

	t.Run("simple InboxFetch", func(t *testing.T) {

		myTs := &testServer{}

		myTs.FetchResponse = makeFetchResponse()
		fxC, _, _ := makeClientServer(t, myTs)
		msgs, _, err := fxC.InboxFetch(ctx, "")

		require.NoError(t, err)
		assert.Len(t, msgs, 10)

	})
}

func TestInbox_Notify(t *testing.T) {
	var makeClientServer = func(t *testing.T, ts *testServer) (fxC, fxS *fixture, peerId string) {
		fxC = newFixture(t, nil)
		fxS = newFixtureWithReceiver(t, ts)
		peerId = "peer"
		identity, err := fxC.account.Account().SignKey.GetPublic().Marshall()
		require.NoError(t, err)
		mcS, mcC := rpctest.MultiConnPairWithIdentity(peerId, peerId+"client", identity)
		pS, err := peer.NewPeer(mcS, fxC.ts)
		require.NoError(t, err)
		fxC.tp.AddPeer(ctx, pS)
		_, err = peer.NewPeer(mcC, fxS.ts)
		require.NoError(t, err)
		return
	}
	t.Run("notify simple test", func(t *testing.T) {

		myTs := &testServer{
			name:             "fxC",
			NotifySenderChan: make(chan *coordinatorproto.InboxNotifySubscribeEvent),
		}
		expectedEvent := &coordinatorproto.InboxNotifySubscribeEvent{
			NotifyId: "hello",
		}
		_, fxS, _ := makeClientServer(t, myTs)
		var wg sync.WaitGroup
		wg.Add(1)
		fxS.mockReceiver.EXPECT().
			Receive(expectedEvent).
			Do(func(evt *coordinatorproto.InboxNotifySubscribeEvent) {
				defer wg.Done()
			}).
			Times(1)

		myTs.NotifySenderChan <- expectedEvent

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			return
		case <-time.After(2 * time.Second):
			t.Fatal("Receive callback was not triggered in time")
		}
	})

	t.Run("InboxAddMessage sends notification and InboxFetch gets new message", func(t *testing.T) {

		myTs := &testServer{
			name:             "fxC",
			NotifySenderChan: make(chan *coordinatorproto.InboxNotifySubscribeEvent),
		}
		expectedEvent := &coordinatorproto.InboxNotifySubscribeEvent{
			NotifyId: "event",
		}

		myTs.FetchResponse = makeFetchResponse()

		fxC, fxS, _ := makeClientServer(t, myTs)
		var wg sync.WaitGroup
		wg.Add(1)
		fxS.mockReceiver.EXPECT().
			Receive(expectedEvent).
			Do(func(evt *coordinatorproto.InboxNotifySubscribeEvent) {
				defer wg.Done()
			}).
			Times(1)

		privKey, pubKey, _ := crypto.GenerateEd25519Key(rand.Reader)
		msg := &coordinatorproto.InboxMessage{
			Packet: &coordinatorproto.InboxPacket{
				Payload: &coordinatorproto.InboxPayload{
					Body: []byte("hello, notify testId"),
				},
			},
		}

		fxC.InboxAddMessage(ctx, pubKey, msg)

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			msgs, _, err := fxC.InboxFetch(ctx, "")
			require.NoError(t, err)
			assert.Len(t, msgs, 11)

			msg, err := privKey.Decrypt(msgs[10].Packet.Payload.Body)
			require.NoError(t, err)
			assert.Equal(t, "hello, notify testId", string(msg))

			return
		case <-time.After(2 * time.Second):
			t.Fatal("Receive callback was not triggered in time")
		}
	})

}

func makeFetchResponse() *coordinatorproto.InboxFetchResponse {
	res := new(coordinatorproto.InboxFetchResponse)
	res.Messages = make([]*coordinatorproto.InboxMessage, 10)
	// message.Packet.ReceiverIdentity
	//
	for i := range 10 {
		res.Messages[i] = &coordinatorproto.InboxMessage{}
	}
	return res
}

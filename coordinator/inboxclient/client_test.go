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

func makeClientServer(t *testing.T) (fxC *fixtureClient, fxS *fixtureServer, peerId string) {
	fxC = newFixtureClient(t)
	fxS = newFixtureServer(t)
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

func TestInbox_Fetch(t *testing.T) {
	t.Run("simple InboxFetch", func(t *testing.T) {
		fxC, fxS, _ := makeClientServer(t)
		fxS.testServer.FetchResponse = makeFetchResponse()

		msgs, _, err := fxC.inbox.InboxFetch(ctx, "")
		require.NoError(t, err)
		assert.Len(t, msgs, 10)
	})
}

func TestInbox_Notify(t *testing.T) {
	t.Run("notify simple test", func(t *testing.T) {
		expectedEvent := &coordinatorproto.NotifySubscribeEvent{
			EventType: coordinatorproto.NotifyEventType_InboxNewMessageEvent,
			Payload:   []byte("hello"),
		}
		fxC, fxS, _ := makeClientServer(t)
		var wg sync.WaitGroup
		wg.Add(1)
		fxC.mockReceiver.EXPECT().
			Receive(expectedEvent).
			Do(func(evt *coordinatorproto.NotifySubscribeEvent) {
				defer wg.Done()
			}).
			Times(1)

		fxS.testServer.NotifySenderChan <- expectedEvent

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
		fxC, fxS, _ := makeClientServer(t)
		fxS.testServer.FetchResponse = makeFetchResponse()
		expectedEvent := &coordinatorproto.NotifySubscribeEvent{
			EventType: coordinatorproto.NotifyEventType_InboxNewMessageEvent,
			Payload:   []byte("hello"),
		}

		var wg sync.WaitGroup
		wg.Add(1)
		fxC.mockReceiver.EXPECT().
			Receive(expectedEvent).
			Do(func(evt *coordinatorproto.NotifySubscribeEvent) {
				defer wg.Done()
			}).
			Times(1)

		privKey, pubKey, _ := crypto.GenerateEd25519Key(rand.Reader)
		msg := &coordinatorproto.InboxMessage{
			Id: "hello",
			Packet: &coordinatorproto.InboxPacket{
				Payload: &coordinatorproto.InboxPayload{
					Body: []byte("hello, notify testId"),
				},
			},
		}

		fxC.inbox.InboxAddMessage(ctx, pubKey, msg)

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			msgs, _, err := fxC.inbox.InboxFetch(ctx, "")
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
	for i := range 10 {
		res.Messages[i] = &coordinatorproto.InboxMessage{}
	}
	return res
}

package inboxclient

import (
	"context"
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

func TestInbox_CryptoTest(t *testing.T) {
	privKey, pubKey, _ := crypto.GenerateRandomEd25519KeyPair()

	t.Run("test encrypt/decrypt", func(t *testing.T) {
		body := []byte("hello")
		encrypted, _ := pubKey.Encrypt(body)
		decrypted, _ := privKey.Decrypt(encrypted)
		assert.Equal(t, body, decrypted)
	})
}

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

		res := new(coordinatorproto.InboxFetchResponse)
		res.Messages = make([]*coordinatorproto.InboxMessage, 10)
		for i := range 10 {
			res.Messages[i] = &coordinatorproto.InboxMessage{}
		}
		myTs.FetchResponse = res
		fxC, _, _ := makeClientServer(t, myTs)
		msgs, err := fxC.InboxFetch(ctx, "")

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

}

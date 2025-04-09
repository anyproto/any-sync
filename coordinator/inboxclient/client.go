package inboxclient

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"storj.io/drpc"

	commonaccount "github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
	"github.com/anyproto/any-sync/coordinator/subscribeclient"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/util/crypto"
	"go.uber.org/zap"
)

const CName = "common.inboxclient"

type MessageReceiver func(message *coordinatorproto.InboxNotifySubscribeEvent)

var (
	ErrPubKeyMissing     = errors.New("peer pub key missing")
	ErrNetworkMismatched = errors.New("network mismatched")
)

var log = logger.NewNamed(CName)

type inboxClient struct {
	nodeconf        nodeconf.Service
	pool            pool.Pool
	account         commonaccount.Service
	subscribeClient subscribeclient.SubscribeClientService
	mu              sync.Mutex
	close           chan struct{}

	running         bool
	messageReceiver MessageReceiver
}

func New() InboxClient {
	return new(inboxClient)
}

type InboxClient interface {
	InboxFetch(ctx context.Context, offset string) (messages []*coordinatorproto.InboxMessage, err error)
	InboxAddMessage(ctx context.Context, receiverPubKey crypto.PubKey, message *coordinatorproto.InboxMessage) (err error)
	SetMessageReceiver(receiver MessageReceiver) (err error)
	app.ComponentRunnable
}

func (c *inboxClient) Name() (name string) {
	return CName
}

func (c *inboxClient) Init(a *app.App) (err error) {
	c.pool = a.MustComponent(pool.CName).(pool.Pool)
	c.nodeconf = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	c.account = a.MustComponent(commonaccount.CName).(commonaccount.Service)
	c.subscribeClient = a.MustComponent(subscribeclient.CName).(subscribeclient.SubscribeClientService)
	c.close = make(chan struct{})

	return nil
}

func (c *inboxClient) SetMessageReceiver(receiver MessageReceiver) (err error) {
	if c.running {
		return fmt.Errorf("set receiver must be called before Run")
	}

	c.subscribeClient.Subscribe(coordinatorproto.NotifyEventType_InboxNewMessageEvent, func(event *coordinatorproto.NotifySubscribeEvent) {
		inboxEvent := event.GetInboxEvent()
		if inboxEvent == nil {
			err = fmt.Errorf("inboxEvent is nil. Original event: %#v", event)
		}
		receiver(inboxEvent)
	})
	return
}

func (c *inboxClient) Run(ctx context.Context) error {
	c.running = true
	if !c.subscribeClient.IsSubscribed(CName, coordinatorproto.NotifyEventType_InboxNewMessageEvent) {
		err := fmt.Errorf("messageReceiver is nil: it must be set to get inbox notifications")
		return err
	}

	return nil
}

func (c *inboxClient) Close(_ context.Context) error {
	return nil
}

// Fetches inbox messages for requster accountId.
// `offset` is `id` of the latest message fetched, i.e. response will
// contain messages with `timestamp` after this `id`, or all messages if empty.
//
// It is assumed that caller will save the last message id somewhere locally to reuse it
// in the next call.
func (c *inboxClient) InboxFetch(ctx context.Context, offset string) (messages []*coordinatorproto.InboxMessage, err error) {
	log.Debug("inbox fetch", zap.String("offset", offset))
	err = c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		resp, err := cl.InboxFetch(ctx, &coordinatorproto.InboxFetchRequest{
			Offset: offset,
		})
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		messages = resp.Messages
		return nil
	})
	return

}

func (c *inboxClient) InboxAddMessage(ctx context.Context, receiverPubKey crypto.PubKey, message *coordinatorproto.InboxMessage) (err error) {
	encrypted, err := receiverPubKey.Encrypt(message.Packet.Payload.Body)
	if err != nil {
		return
	}
	message.Packet.Payload.Body = encrypted
	signature, err := c.account.Account().SignKey.Sign(message.Packet.Payload.Body)
	if err != nil {
		return fmt.Errorf("sign body: %w", err)
	}
	message.Packet.SenderSignature = signature

	err = c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		_, err := cl.InboxAddMessage(ctx, &coordinatorproto.InboxAddMessageRequest{
			Message: message,
		})
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return

}

func (c *inboxClient) getPeer(ctx context.Context) (peer.Peer, error) {
	p, err := c.pool.GetOneOf(ctx, c.nodeconf.CoordinatorPeers())
	if err != nil {
		return nil, err
	}
	pubKey, err := peer.CtxPubKey(p.Context())
	if err != nil {
		return nil, ErrPubKeyMissing
	}
	if pubKey.Network() != c.nodeconf.Configuration().NetworkId {
		return nil, ErrNetworkMismatched
	}
	return p, nil
}

func (c *inboxClient) doClient(ctx context.Context, f func(cl coordinatorproto.DRPCCoordinatorClient) error) error {
	p, err := c.getPeer(ctx)
	if err != nil {
		return err
	}
	return p.DoDrpc(ctx, func(conn drpc.Conn) error {
		return f(coordinatorproto.NewDRPCCoordinatorClient(conn))
	})
}

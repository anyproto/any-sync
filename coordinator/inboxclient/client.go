package inboxclient

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"storj.io/drpc"

	commonaccount "github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/util/crypto"
	"go.uber.org/zap"
)

const CName = "common.inboxclient"

type MessageReceiver func(messages []*coordinatorproto.InboxMessage)

var (
	ErrPubKeyMissing     = errors.New("peer pub key missing")
	ErrNetworkMismatched = errors.New("network mismatched")
)

var log = logger.NewNamed(CName)

type inboxClient struct {
	nodeconf nodeconf.Service
	pool     pool.Pool
	account  commonaccount.Service

	mu     sync.Mutex
	close  chan struct{}
	stream *stream

	offset string

	running         bool
	messageReceiver MessageReceiver
}

func New() InboxClient {
	return new(inboxClient)
}

type InboxClient interface {
	InboxFetch(ctx context.Context, offset string) (records []*coordinatorproto.InboxMessage, err error)
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

	c.close = make(chan struct{})

	return nil
}

func (c *inboxClient) SetMessageReceiver(receiver MessageReceiver) (err error) {
	if c.running {
		return fmt.Errorf("set receiver must be called before Run")
	}
	c.messageReceiver = receiver
	return
}

func (c *inboxClient) Run(ctx context.Context) error {
	c.running = true
	if c.messageReceiver != nil {
		go c.streamWatcher()
	} else {
		err := fmt.Errorf("messageReceiver is nil: can't start streamWatcher()")
		return err
	}
	return nil
}

func (c *inboxClient) Close(_ context.Context) error {
	c.mu.Lock()
	if c.stream != nil {
		_ = c.stream.Close()
	}
	c.mu.Unlock()
	select {
	case <-c.close:
	default:
		close(c.close)
	}
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
	log.Debug("receiverId", zap.String("id", message.Packet.ReceiverIdentity))
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

func (c *inboxClient) openStream(ctx context.Context) (st *stream, err error) {
	pr, err := c.pool.GetOneOf(ctx, c.nodeconf.CoordinatorPeers())
	if err != nil {
		return nil, err
	}
	pr.SetTTL(time.Hour * 24)
	dc, err := pr.AcquireDrpcConn(ctx)
	if err != nil {
		return nil, err
	}
	req := &coordinatorproto.InboxNotifySubscribeRequest{}
	rpcStream, err := coordinatorproto.NewDRPCCoordinatorClient(dc).InboxNotifySubscribe(ctx, req)
	if err != nil {
		return nil, rpcerr.Unwrap(err)
	}
	return runStream(rpcStream), nil
}

func (c *inboxClient) streamWatcher() {
	var (
		err error
		st  *stream
	)
	for {
		log.Info("streamWatcher: open inbox stream")
		// open stream
		if st, err = c.openStream(context.Background()); err != nil {
			log.Error("failed to open inbox notification stream", zap.Error(err))
			return
		}
		c.mu.Lock()
		c.stream = st
		c.mu.Unlock()
		// read stream
		if err = c.streamReader(); err != nil {
			log.Error("stream read error", zap.Error(err))
			continue
		}
		return
	}
}

func (c *inboxClient) streamReader() error {
	for {
		event := c.stream.WaitNotifyEvents()
		if event == nil {
			continue
		}

		msgs, err := c.InboxFetch(context.TODO(), c.offset)
		if err != nil {
			log.Error("fetching after notify err", zap.Error(err))
		}
		if len(msgs) != 0 {
			// assuming that msgs are sorted
			c.offset = msgs[len(msgs)-1].Id
			for _, msg := range msgs {
				encrypted := msg.Packet.Payload.Body
				body, err := c.account.Account().SignKey.Decrypt(encrypted)
				if err != nil {
					log.Error("error decrypting body", zap.Error(err))
				}
				msg.Packet.Payload.Body = body
			}
			c.messageReceiver(msgs)
		} else {
			log.Error("fetched zero msgs after notify")
		}
	}
}

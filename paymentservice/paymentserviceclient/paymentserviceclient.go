package paymentserviceclient

import (
	"context"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
	"github.com/anyproto/any-sync/nodeconf"

	pp "github.com/anyproto/any-sync/paymentservice/paymentserviceproto"
)

const CName = "any-pp.drpcclient"

/*
 * This client component can be used to access the Any Payment Processing node
 * from other components.
 */
type AnyPpClientService interface {
	GetSubscriptionStatus(ctx context.Context, in *pp.GetSubscriptionRequestSigned) (out *pp.GetSubscriptionResponse, err error)
	BuySubscription(ctx context.Context, in *pp.BuySubscriptionRequestSigned) (out *pp.BuySubscriptionResponse, err error)

	app.ComponentRunnable
}

type service struct {
	pool     pool.Pool
	nodeconf nodeconf.Service
	close    chan struct{}
}

func (s *service) Init(a *app.App) (err error) {
	s.pool = a.MustComponent(pool.CName).(pool.Pool)
	s.nodeconf = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	s.close = make(chan struct{})
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func New() AnyPpClientService {
	return new(service)
}

func (s *service) Run(_ context.Context) error {
	return nil
}

func (s *service) Close(_ context.Context) error {
	select {
	case <-s.close:
	default:
		close(s.close)
	}
	return nil
}

func (s *service) doClient(ctx context.Context, fn func(cl pp.DRPCAnyPaymentProcessingClient) error) error {
	// it will try to connect to the Payment Node
	// please use "paymentProcessingNode" type of node in the config (in the network.nodes array)
	peer, err := s.pool.Get(ctx, s.nodeconf.PaymentProcessingNodePeers()[0])

	if err != nil {
		return err
	}

	dc, err := peer.AcquireDrpcConn(ctx)
	if err != nil {
		return err
	}
	defer peer.ReleaseDrpcConn(dc)

	return fn(pp.NewDRPCAnyPaymentProcessingClient(dc))
}

func (s *service) GetSubscriptionStatus(ctx context.Context, in *pp.GetSubscriptionRequestSigned) (out *pp.GetSubscriptionResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessingClient) error {
		if out, err = cl.GetSubscriptionStatus(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) BuySubscription(ctx context.Context, in *pp.BuySubscriptionRequestSigned) (out *pp.BuySubscriptionResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessingClient) error {
		if out, err = cl.BuySubscription(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

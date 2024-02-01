package paymentserviceclient

import (
	"context"
	"errors"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
	"github.com/anyproto/any-sync/nodeconf"

	pp "github.com/anyproto/any-sync/paymentservice/paymentserviceproto"
)

const CName = "any-pp.drpcclient"

var log = logger.NewNamed(CName)

/*
 * This client component can be used to access the Any Payment Processing node
 * from other components.
 */
type AnyPpClientService interface {
	GetSubscriptionStatus(ctx context.Context, in *pp.GetSubscriptionRequestSigned) (out *pp.GetSubscriptionResponse, err error)
	BuySubscription(ctx context.Context, in *pp.BuySubscriptionRequestSigned) (out *pp.BuySubscriptionResponse, err error)
	GetSubscriptionPortalLink(ctx context.Context, in *pp.GetSubscriptionPortalLinkRequestSigned) (out *pp.GetSubscriptionPortalLinkResponse, err error)

	app.Component
}

type service struct {
	pool     pool.Pool
	nodeconf nodeconf.Service
}

func (s *service) Init(a *app.App) (err error) {
	s.pool = a.MustComponent(pool.CName).(pool.Pool)
	s.nodeconf = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func New() AnyPpClientService {
	return new(service)
}

func (s *service) doClient(ctx context.Context, fn func(cl pp.DRPCAnyPaymentProcessingClient) error) error {
	if len(s.nodeconf.PaymentProcessingNodePeers()) == 0 {
		log.Error("no payment processing peers configured")

		return errors.New("no paymentProcessingNode peers configured")
	}

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

func (s *service) GetSubscriptionPortalLink(ctx context.Context, in *pp.GetSubscriptionPortalLinkRequestSigned) (out *pp.GetSubscriptionPortalLinkResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessingClient) error {
		if out, err = cl.GetSubscriptionPortalLink(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

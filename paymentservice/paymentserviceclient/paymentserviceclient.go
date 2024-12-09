//go:generate mockgen -destination=mock/mock_paymentserviceclient.go -package=mock_paymentserviceclient github.com/anyproto/any-sync/paymentservice/paymentserviceclient AnyPpClientService
package paymentserviceclient

import (
	"context"
	"errors"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
	"github.com/anyproto/any-sync/nodeconf"
	"go.uber.org/zap"

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
	IsNameValid(ctx context.Context, in *pp.IsNameValidRequest) (out *pp.IsNameValidResponse, err error)
	BuySubscription(ctx context.Context, in *pp.BuySubscriptionRequestSigned) (out *pp.BuySubscriptionResponse, err error)
	GetSubscriptionPortalLink(ctx context.Context, in *pp.GetSubscriptionPortalLinkRequestSigned) (out *pp.GetSubscriptionPortalLinkResponse, err error)
	GetVerificationEmail(ctx context.Context, in *pp.GetVerificationEmailRequestSigned) (out *pp.GetVerificationEmailResponse, err error)
	VerifyEmail(ctx context.Context, in *pp.VerifyEmailRequestSigned) (out *pp.VerifyEmailResponse, err error)
	FinalizeSubscription(ctx context.Context, in *pp.FinalizeSubscriptionRequestSigned) (out *pp.FinalizeSubscriptionResponse, err error)
	GetAllTiers(ctx context.Context, in *pp.GetTiersRequestSigned) (out *pp.GetTiersResponse, err error)
	VerifyAppStoreReceipt(ctx context.Context, in *pp.VerifyAppStoreReceiptRequestSigned) (out *pp.VerifyAppStoreReceiptResponse, err error)

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

/*
 * Case 1: Custom network, no paymentProcessingNode peers ->
 *  This should not be called in custom networks! Please see what client of this service is doing.
 *  Otherwise will return: { "no payment processing peers configured. Maybe you're on a custom network" }
 *
 * Case 2: Anytype network, no paymentProcessingNode peers in config ->
 *  !!! This is a big issue, probably because of problems with nodeconf. Should be logged!
 *
 * Case 3: Anytype network, paymentProcessingNode peers in config, no connectivity ->
 *  This can happen due to network connectivity issues and it is OK.
 *  Will return:
 *  { "failed to get a paymentnode peer. maybe you're on a custom network","error":"unable to connect” }
 */
func (s *service) doClient(ctx context.Context, fn func(cl pp.DRPCAnyPaymentProcessingClient) error) error {
	if len(s.nodeconf.PaymentProcessingNodePeers()) == 0 {
		return errors.New("no paymentProcessingNode peers configured. Maybe you're on a custom network. Node config ID: " + s.nodeconf.Id())
	}

	// it will try to connect to the Payment Node
	// please use "paymentProcessingNode" type of node in the config (in the network.nodes array)
	peer, err := s.pool.GetOneOf(ctx, s.nodeconf.PaymentProcessingNodePeers())
	if err != nil {
		log.Error("failed to get a paymentnode peer", zap.Error(err))
		return err
	}

	log.Debug("trying to connect to paymentProcessingNode peer: ", zap.Any("peer", peer))

	dc, err := peer.AcquireDrpcConn(ctx)
	if err != nil {
		log.Error("failed to acquire a DRPC connection to paymentnode", zap.Error(err))
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

func (s *service) GetVerificationEmail(ctx context.Context, in *pp.GetVerificationEmailRequestSigned) (out *pp.GetVerificationEmailResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessingClient) error {
		if out, err = cl.GetVerificationEmail(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) VerifyEmail(ctx context.Context, in *pp.VerifyEmailRequestSigned) (out *pp.VerifyEmailResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessingClient) error {
		if out, err = cl.VerifyEmail(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) FinalizeSubscription(ctx context.Context, in *pp.FinalizeSubscriptionRequestSigned) (out *pp.FinalizeSubscriptionResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessingClient) error {
		if out, err = cl.FinalizeSubscription(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) GetAllTiers(ctx context.Context, in *pp.GetTiersRequestSigned) (out *pp.GetTiersResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessingClient) error {
		if out, err = cl.GetAllTiers(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) IsNameValid(ctx context.Context, in *pp.IsNameValidRequest) (out *pp.IsNameValidResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessingClient) error {
		if out, err = cl.IsNameValid(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) VerifyAppStoreReceipt(ctx context.Context, in *pp.VerifyAppStoreReceiptRequestSigned) (out *pp.VerifyAppStoreReceiptResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessingClient) error {
		if out, err = cl.VerifyAppStoreReceipt(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

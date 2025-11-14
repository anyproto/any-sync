//go:generate mockgen -destination=mock/mock_paymentserviceclient2.go -package=mock_paymentserviceclient2 github.com/anyproto/any-sync/paymentservice/paymentserviceclient2 AnyPpClientServiceV2
package paymentserviceclient2

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

const CName = "any-pp.drpcclient2"

var log = logger.NewNamed(CName)

type AnyPpClientServiceV2 interface {
	GetProducts(ctx context.Context, in *pp.MembershipV2_GetProductsRequest) (out *pp.MembershipV2_GetProductsResponse, err error)

	GetStatus(ctx context.Context, in *pp.MembershipV2_GetStatusRequest) (out *pp.MembershipV2_GetStatusResponse, err error)

	WebAuth(ctx context.Context, in *pp.MembershipV2_WebAuthRequest) (out *pp.MembershipV2_WebAuthResponse, err error)

	AnyNameIsValid(ctx context.Context, in *pp.MembershipV2_AnyNameIsValidRequest) (out *pp.MembershipV2_AnyNameIsValidResponse, err error)

	AnyNameAllocate(ctx context.Context, in *pp.MembershipV2_AnyNameAllocateRequest) (out *pp.MembershipV2_AnyNameAllocateResponse, err error)

	StoreCartGet(ctx context.Context, in *pp.MembershipV2_StoreCartGetRequest) (out *pp.MembershipV2_StoreCartGetResponse, err error)
	StoreCartUpdate(ctx context.Context, in *pp.MembershipV2_StoreCartUpdateRequest) (out *pp.MembershipV2_StoreCartUpdateResponse, err error)

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

func New() AnyPpClientServiceV2 {
	return new(service)
}

func (s *service) doClient(ctx context.Context, fn func(cl pp.DRPCAnyPaymentProcessingV2Client) error) error {
	if len(s.nodeconf.PaymentProcessingNodePeers()) == 0 {
		log.Error("no payment processing peers configured")
		return errors.New("no paymentProcessingNode peers configured. Node config ID: " + s.nodeconf.Id())
	}

	// it will try to connect to the Payment Node
	// please use "paymentProcessingNode" type of node in the config (in the network.nodes array)
	peer, err := s.pool.GetOneOf(ctx, s.nodeconf.PaymentProcessingNodePeers())
	if err != nil {
		log.Error("failed to get a paymentnode peer. maybe you're on a custom network", zap.Error(err))
		return err
	}

	log.Debug("trying to connect to paymentProcessingNode peer: ", zap.Any("peer", peer))

	dc, err := peer.AcquireDrpcConn(ctx)
	if err != nil {
		log.Error("failed to acquire a DRPC connection to paymentnode", zap.Error(err))
		return err
	}
	defer peer.ReleaseDrpcConn(ctx, dc)

	return fn(pp.NewDRPCAnyPaymentProcessingV2Client(dc))
}

func (s *service) GetProducts(ctx context.Context, in *pp.MembershipV2_GetProductsRequest) (out *pp.MembershipV2_GetProductsResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessingV2Client) error {
		if out, err = cl.GetProducts(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) GetStatus(ctx context.Context, in *pp.MembershipV2_GetStatusRequest) (out *pp.MembershipV2_GetStatusResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessingV2Client) error {
		if out, err = cl.GetStatus(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) WebAuth(ctx context.Context, in *pp.MembershipV2_WebAuthRequest) (out *pp.MembershipV2_WebAuthResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessingV2Client) error {
		if out, err = cl.WebAuth(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) AnyNameIsValid(ctx context.Context, in *pp.MembershipV2_AnyNameIsValidRequest) (out *pp.MembershipV2_AnyNameIsValidResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessingV2Client) error {
		if out, err = cl.AnyNameIsValid(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) AnyNameAllocate(ctx context.Context, in *pp.MembershipV2_AnyNameAllocateRequest) (out *pp.MembershipV2_AnyNameAllocateResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessingV2Client) error {
		if out, err = cl.AnyNameAllocate(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) StoreCartGet(ctx context.Context, in *pp.MembershipV2_StoreCartGetRequest) (out *pp.MembershipV2_StoreCartGetResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessingV2Client) error {
		if out, err = cl.StoreCartGet(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) StoreCartUpdate(ctx context.Context, in *pp.MembershipV2_StoreCartUpdateRequest) (out *pp.MembershipV2_StoreCartUpdateResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessingV2Client) error {
		if out, err = cl.StoreCartUpdate(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

//go:generate mockgen -destination=mock/mock_paymentserviceclient2.go -package=mock_paymentserviceclient2 github.com/anyproto/any-sync/paymentservice/paymentserviceclient2 AnyPpClientService2
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

type AnyPpClientService2 interface {
	StoreCartGet(ctx context.Context, in *pp.Membership2_StoreCartGetRequest) (out *pp.Membership2_StoreCartResponse, err error)
	StoreCartProductAdd(ctx context.Context, in *pp.Membership2_StoreCartProductAddRequest) (out *pp.Membership2_StoreCartResponse, err error)
	StoreCartProductRemove(ctx context.Context, in *pp.Membership2_StoreCartProductRemoveRequest) (out *pp.Membership2_StoreCartResponse, err error)
	StoreCartPromocodeApply(ctx context.Context, in *pp.Membership2_StoreCartPromocodeApplyRequest) (out *pp.Membership2_StoreCartResponse, err error)
	StoreCartCheckoutGenerate(ctx context.Context, in *pp.Membership2_StoreCartCheckoutGenerateRequest) (out *pp.Membership2_StoreCartCheckoutGenerateResponse, err error)

	ProductsEnumerate(ctx context.Context, in *pp.Membership2_StoreProductsEnumerateRequest) (out *pp.Membership2_StoreProductsEnumerateResponse, err error)
	GetStatus(ctx context.Context, in *pp.Membership2_GetStatusRequest) (out *pp.Membership2_GetStatusResponse, err error)
	ProductAllocateToSpace(ctx context.Context, in *pp.Membership2_ProductAllocateToSpaceRequest) (out *pp.Membership2_ProductAllocateToSpaceResponse, err error)

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

func New() AnyPpClientService2 {
	return new(service)
}

func (s *service) doClient(ctx context.Context, fn func(cl pp.DRPCAnyPaymentProcessing2Client) error) error {
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
	defer peer.ReleaseDrpcConn(dc)

	return fn(pp.NewDRPCAnyPaymentProcessing2Client(dc))
}

func (s *service) GetStatus(ctx context.Context, in *pp.Membership2_GetStatusRequest) (out *pp.Membership2_GetStatusResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessing2Client) error {
		if out, err = cl.GetStatus(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) ProductsEnumerate(ctx context.Context, in *pp.Membership2_StoreProductsEnumerateRequest) (out *pp.Membership2_StoreProductsEnumerateResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessing2Client) error {
		if out, err = cl.ProductsEnumerate(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) StoreCartGet(ctx context.Context, in *pp.Membership2_StoreCartGetRequest) (out *pp.Membership2_StoreCartResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessing2Client) error {
		if out, err = cl.StoreCartGet(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) StoreCartProductAdd(ctx context.Context, in *pp.Membership2_StoreCartProductAddRequest) (out *pp.Membership2_StoreCartResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessing2Client) error {
		if out, err = cl.StoreCartProductAdd(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) StoreCartProductRemove(ctx context.Context, in *pp.Membership2_StoreCartProductRemoveRequest) (out *pp.Membership2_StoreCartResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessing2Client) error {
		if out, err = cl.StoreCartProductRemove(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) StoreCartPromocodeApply(ctx context.Context, in *pp.Membership2_StoreCartPromocodeApplyRequest) (out *pp.Membership2_StoreCartResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessing2Client) error {
		if out, err = cl.StoreCartPromocodeApply(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) StoreCartCheckoutGenerate(ctx context.Context, in *pp.Membership2_StoreCartCheckoutGenerateRequest) (out *pp.Membership2_StoreCartCheckoutGenerateResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessing2Client) error {
		if out, err = cl.StoreCartCheckoutGenerate(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) ProductAllocateToSpace(ctx context.Context, in *pp.Membership2_ProductAllocateToSpaceRequest) (out *pp.Membership2_ProductAllocateToSpaceResponse, err error) {
	err = s.doClient(ctx, func(cl pp.DRPCAnyPaymentProcessing2Client) error {
		if out, err = cl.ProductAllocateToSpace(ctx, in); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

// Code generated by protoc-gen-go-drpc. DO NOT EDIT.
// protoc-gen-go-drpc version: (devel)
// source: paymentservice/paymentserviceproto/protos/paymentservice.proto

package paymentserviceproto

import (
	context "context"
	errors "errors"
	drpc1 "github.com/planetscale/vtprotobuf/codec/drpc"
	drpc "storj.io/drpc"
	drpcerr "storj.io/drpc/drpcerr"
)

type drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto struct{}

func (drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto) Marshal(msg drpc.Message) ([]byte, error) {
	return drpc1.Marshal(msg)
}

func (drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto) Unmarshal(buf []byte, msg drpc.Message) error {
	return drpc1.Unmarshal(buf, msg)
}

func (drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto) JSONMarshal(msg drpc.Message) ([]byte, error) {
	return drpc1.JSONMarshal(msg)
}

func (drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto) JSONUnmarshal(buf []byte, msg drpc.Message) error {
	return drpc1.JSONUnmarshal(buf, msg)
}

type DRPCAnyPaymentProcessingClient interface {
	DRPCConn() drpc.Conn

	GetSubscriptionStatus(ctx context.Context, in *GetSubscriptionRequestSigned) (*GetSubscriptionResponse, error)
	IsNameValid(ctx context.Context, in *IsNameValidRequest) (*IsNameValidResponse, error)
	BuySubscription(ctx context.Context, in *BuySubscriptionRequestSigned) (*BuySubscriptionResponse, error)
	FinalizeSubscription(ctx context.Context, in *FinalizeSubscriptionRequestSigned) (*FinalizeSubscriptionResponse, error)
	GetSubscriptionPortalLink(ctx context.Context, in *GetSubscriptionPortalLinkRequestSigned) (*GetSubscriptionPortalLinkResponse, error)
	GetVerificationEmail(ctx context.Context, in *GetVerificationEmailRequestSigned) (*GetVerificationEmailResponse, error)
	VerifyEmail(ctx context.Context, in *VerifyEmailRequestSigned) (*VerifyEmailResponse, error)
	GetAllTiers(ctx context.Context, in *GetTiersRequestSigned) (*GetTiersResponse, error)
	VerifyAppStoreReceipt(ctx context.Context, in *VerifyAppStoreReceiptRequestSigned) (*VerifyAppStoreReceiptResponse, error)
}

type drpcAnyPaymentProcessingClient struct {
	cc drpc.Conn
}

func NewDRPCAnyPaymentProcessingClient(cc drpc.Conn) DRPCAnyPaymentProcessingClient {
	return &drpcAnyPaymentProcessingClient{cc}
}

func (c *drpcAnyPaymentProcessingClient) DRPCConn() drpc.Conn { return c.cc }

func (c *drpcAnyPaymentProcessingClient) GetSubscriptionStatus(ctx context.Context, in *GetSubscriptionRequestSigned) (*GetSubscriptionResponse, error) {
	out := new(GetSubscriptionResponse)
	err := c.cc.Invoke(ctx, "/AnyPaymentProcessing/GetSubscriptionStatus", drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{}, in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *drpcAnyPaymentProcessingClient) IsNameValid(ctx context.Context, in *IsNameValidRequest) (*IsNameValidResponse, error) {
	out := new(IsNameValidResponse)
	err := c.cc.Invoke(ctx, "/AnyPaymentProcessing/IsNameValid", drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{}, in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *drpcAnyPaymentProcessingClient) BuySubscription(ctx context.Context, in *BuySubscriptionRequestSigned) (*BuySubscriptionResponse, error) {
	out := new(BuySubscriptionResponse)
	err := c.cc.Invoke(ctx, "/AnyPaymentProcessing/BuySubscription", drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{}, in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *drpcAnyPaymentProcessingClient) FinalizeSubscription(ctx context.Context, in *FinalizeSubscriptionRequestSigned) (*FinalizeSubscriptionResponse, error) {
	out := new(FinalizeSubscriptionResponse)
	err := c.cc.Invoke(ctx, "/AnyPaymentProcessing/FinalizeSubscription", drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{}, in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *drpcAnyPaymentProcessingClient) GetSubscriptionPortalLink(ctx context.Context, in *GetSubscriptionPortalLinkRequestSigned) (*GetSubscriptionPortalLinkResponse, error) {
	out := new(GetSubscriptionPortalLinkResponse)
	err := c.cc.Invoke(ctx, "/AnyPaymentProcessing/GetSubscriptionPortalLink", drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{}, in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *drpcAnyPaymentProcessingClient) GetVerificationEmail(ctx context.Context, in *GetVerificationEmailRequestSigned) (*GetVerificationEmailResponse, error) {
	out := new(GetVerificationEmailResponse)
	err := c.cc.Invoke(ctx, "/AnyPaymentProcessing/GetVerificationEmail", drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{}, in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *drpcAnyPaymentProcessingClient) VerifyEmail(ctx context.Context, in *VerifyEmailRequestSigned) (*VerifyEmailResponse, error) {
	out := new(VerifyEmailResponse)
	err := c.cc.Invoke(ctx, "/AnyPaymentProcessing/VerifyEmail", drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{}, in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *drpcAnyPaymentProcessingClient) GetAllTiers(ctx context.Context, in *GetTiersRequestSigned) (*GetTiersResponse, error) {
	out := new(GetTiersResponse)
	err := c.cc.Invoke(ctx, "/AnyPaymentProcessing/GetAllTiers", drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{}, in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *drpcAnyPaymentProcessingClient) VerifyAppStoreReceipt(ctx context.Context, in *VerifyAppStoreReceiptRequestSigned) (*VerifyAppStoreReceiptResponse, error) {
	out := new(VerifyAppStoreReceiptResponse)
	err := c.cc.Invoke(ctx, "/AnyPaymentProcessing/VerifyAppStoreReceipt", drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{}, in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type DRPCAnyPaymentProcessingServer interface {
	GetSubscriptionStatus(context.Context, *GetSubscriptionRequestSigned) (*GetSubscriptionResponse, error)
	IsNameValid(context.Context, *IsNameValidRequest) (*IsNameValidResponse, error)
	BuySubscription(context.Context, *BuySubscriptionRequestSigned) (*BuySubscriptionResponse, error)
	FinalizeSubscription(context.Context, *FinalizeSubscriptionRequestSigned) (*FinalizeSubscriptionResponse, error)
	GetSubscriptionPortalLink(context.Context, *GetSubscriptionPortalLinkRequestSigned) (*GetSubscriptionPortalLinkResponse, error)
	GetVerificationEmail(context.Context, *GetVerificationEmailRequestSigned) (*GetVerificationEmailResponse, error)
	VerifyEmail(context.Context, *VerifyEmailRequestSigned) (*VerifyEmailResponse, error)
	GetAllTiers(context.Context, *GetTiersRequestSigned) (*GetTiersResponse, error)
	VerifyAppStoreReceipt(context.Context, *VerifyAppStoreReceiptRequestSigned) (*VerifyAppStoreReceiptResponse, error)
}

type DRPCAnyPaymentProcessingUnimplementedServer struct{}

func (s *DRPCAnyPaymentProcessingUnimplementedServer) GetSubscriptionStatus(context.Context, *GetSubscriptionRequestSigned) (*GetSubscriptionResponse, error) {
	return nil, drpcerr.WithCode(errors.New("Unimplemented"), drpcerr.Unimplemented)
}

func (s *DRPCAnyPaymentProcessingUnimplementedServer) IsNameValid(context.Context, *IsNameValidRequest) (*IsNameValidResponse, error) {
	return nil, drpcerr.WithCode(errors.New("Unimplemented"), drpcerr.Unimplemented)
}

func (s *DRPCAnyPaymentProcessingUnimplementedServer) BuySubscription(context.Context, *BuySubscriptionRequestSigned) (*BuySubscriptionResponse, error) {
	return nil, drpcerr.WithCode(errors.New("Unimplemented"), drpcerr.Unimplemented)
}

func (s *DRPCAnyPaymentProcessingUnimplementedServer) FinalizeSubscription(context.Context, *FinalizeSubscriptionRequestSigned) (*FinalizeSubscriptionResponse, error) {
	return nil, drpcerr.WithCode(errors.New("Unimplemented"), drpcerr.Unimplemented)
}

func (s *DRPCAnyPaymentProcessingUnimplementedServer) GetSubscriptionPortalLink(context.Context, *GetSubscriptionPortalLinkRequestSigned) (*GetSubscriptionPortalLinkResponse, error) {
	return nil, drpcerr.WithCode(errors.New("Unimplemented"), drpcerr.Unimplemented)
}

func (s *DRPCAnyPaymentProcessingUnimplementedServer) GetVerificationEmail(context.Context, *GetVerificationEmailRequestSigned) (*GetVerificationEmailResponse, error) {
	return nil, drpcerr.WithCode(errors.New("Unimplemented"), drpcerr.Unimplemented)
}

func (s *DRPCAnyPaymentProcessingUnimplementedServer) VerifyEmail(context.Context, *VerifyEmailRequestSigned) (*VerifyEmailResponse, error) {
	return nil, drpcerr.WithCode(errors.New("Unimplemented"), drpcerr.Unimplemented)
}

func (s *DRPCAnyPaymentProcessingUnimplementedServer) GetAllTiers(context.Context, *GetTiersRequestSigned) (*GetTiersResponse, error) {
	return nil, drpcerr.WithCode(errors.New("Unimplemented"), drpcerr.Unimplemented)
}

func (s *DRPCAnyPaymentProcessingUnimplementedServer) VerifyAppStoreReceipt(context.Context, *VerifyAppStoreReceiptRequestSigned) (*VerifyAppStoreReceiptResponse, error) {
	return nil, drpcerr.WithCode(errors.New("Unimplemented"), drpcerr.Unimplemented)
}

type DRPCAnyPaymentProcessingDescription struct{}

func (DRPCAnyPaymentProcessingDescription) NumMethods() int { return 9 }

func (DRPCAnyPaymentProcessingDescription) Method(n int) (string, drpc.Encoding, drpc.Receiver, interface{}, bool) {
	switch n {
	case 0:
		return "/AnyPaymentProcessing/GetSubscriptionStatus", drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{},
			func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
				return srv.(DRPCAnyPaymentProcessingServer).
					GetSubscriptionStatus(
						ctx,
						in1.(*GetSubscriptionRequestSigned),
					)
			}, DRPCAnyPaymentProcessingServer.GetSubscriptionStatus, true
	case 1:
		return "/AnyPaymentProcessing/IsNameValid", drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{},
			func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
				return srv.(DRPCAnyPaymentProcessingServer).
					IsNameValid(
						ctx,
						in1.(*IsNameValidRequest),
					)
			}, DRPCAnyPaymentProcessingServer.IsNameValid, true
	case 2:
		return "/AnyPaymentProcessing/BuySubscription", drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{},
			func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
				return srv.(DRPCAnyPaymentProcessingServer).
					BuySubscription(
						ctx,
						in1.(*BuySubscriptionRequestSigned),
					)
			}, DRPCAnyPaymentProcessingServer.BuySubscription, true
	case 3:
		return "/AnyPaymentProcessing/FinalizeSubscription", drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{},
			func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
				return srv.(DRPCAnyPaymentProcessingServer).
					FinalizeSubscription(
						ctx,
						in1.(*FinalizeSubscriptionRequestSigned),
					)
			}, DRPCAnyPaymentProcessingServer.FinalizeSubscription, true
	case 4:
		return "/AnyPaymentProcessing/GetSubscriptionPortalLink", drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{},
			func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
				return srv.(DRPCAnyPaymentProcessingServer).
					GetSubscriptionPortalLink(
						ctx,
						in1.(*GetSubscriptionPortalLinkRequestSigned),
					)
			}, DRPCAnyPaymentProcessingServer.GetSubscriptionPortalLink, true
	case 5:
		return "/AnyPaymentProcessing/GetVerificationEmail", drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{},
			func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
				return srv.(DRPCAnyPaymentProcessingServer).
					GetVerificationEmail(
						ctx,
						in1.(*GetVerificationEmailRequestSigned),
					)
			}, DRPCAnyPaymentProcessingServer.GetVerificationEmail, true
	case 6:
		return "/AnyPaymentProcessing/VerifyEmail", drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{},
			func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
				return srv.(DRPCAnyPaymentProcessingServer).
					VerifyEmail(
						ctx,
						in1.(*VerifyEmailRequestSigned),
					)
			}, DRPCAnyPaymentProcessingServer.VerifyEmail, true
	case 7:
		return "/AnyPaymentProcessing/GetAllTiers", drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{},
			func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
				return srv.(DRPCAnyPaymentProcessingServer).
					GetAllTiers(
						ctx,
						in1.(*GetTiersRequestSigned),
					)
			}, DRPCAnyPaymentProcessingServer.GetAllTiers, true
	case 8:
		return "/AnyPaymentProcessing/VerifyAppStoreReceipt", drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{},
			func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
				return srv.(DRPCAnyPaymentProcessingServer).
					VerifyAppStoreReceipt(
						ctx,
						in1.(*VerifyAppStoreReceiptRequestSigned),
					)
			}, DRPCAnyPaymentProcessingServer.VerifyAppStoreReceipt, true
	default:
		return "", nil, nil, nil, false
	}
}

func DRPCRegisterAnyPaymentProcessing(mux drpc.Mux, impl DRPCAnyPaymentProcessingServer) error {
	return mux.Register(impl, DRPCAnyPaymentProcessingDescription{})
}

type DRPCAnyPaymentProcessing_GetSubscriptionStatusStream interface {
	drpc.Stream
	SendAndClose(*GetSubscriptionResponse) error
}

type drpcAnyPaymentProcessing_GetSubscriptionStatusStream struct {
	drpc.Stream
}

func (x *drpcAnyPaymentProcessing_GetSubscriptionStatusStream) SendAndClose(m *GetSubscriptionResponse) error {
	if err := x.MsgSend(m, drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{}); err != nil {
		return err
	}
	return x.CloseSend()
}

type DRPCAnyPaymentProcessing_IsNameValidStream interface {
	drpc.Stream
	SendAndClose(*IsNameValidResponse) error
}

type drpcAnyPaymentProcessing_IsNameValidStream struct {
	drpc.Stream
}

func (x *drpcAnyPaymentProcessing_IsNameValidStream) SendAndClose(m *IsNameValidResponse) error {
	if err := x.MsgSend(m, drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{}); err != nil {
		return err
	}
	return x.CloseSend()
}

type DRPCAnyPaymentProcessing_BuySubscriptionStream interface {
	drpc.Stream
	SendAndClose(*BuySubscriptionResponse) error
}

type drpcAnyPaymentProcessing_BuySubscriptionStream struct {
	drpc.Stream
}

func (x *drpcAnyPaymentProcessing_BuySubscriptionStream) SendAndClose(m *BuySubscriptionResponse) error {
	if err := x.MsgSend(m, drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{}); err != nil {
		return err
	}
	return x.CloseSend()
}

type DRPCAnyPaymentProcessing_FinalizeSubscriptionStream interface {
	drpc.Stream
	SendAndClose(*FinalizeSubscriptionResponse) error
}

type drpcAnyPaymentProcessing_FinalizeSubscriptionStream struct {
	drpc.Stream
}

func (x *drpcAnyPaymentProcessing_FinalizeSubscriptionStream) SendAndClose(m *FinalizeSubscriptionResponse) error {
	if err := x.MsgSend(m, drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{}); err != nil {
		return err
	}
	return x.CloseSend()
}

type DRPCAnyPaymentProcessing_GetSubscriptionPortalLinkStream interface {
	drpc.Stream
	SendAndClose(*GetSubscriptionPortalLinkResponse) error
}

type drpcAnyPaymentProcessing_GetSubscriptionPortalLinkStream struct {
	drpc.Stream
}

func (x *drpcAnyPaymentProcessing_GetSubscriptionPortalLinkStream) SendAndClose(m *GetSubscriptionPortalLinkResponse) error {
	if err := x.MsgSend(m, drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{}); err != nil {
		return err
	}
	return x.CloseSend()
}

type DRPCAnyPaymentProcessing_GetVerificationEmailStream interface {
	drpc.Stream
	SendAndClose(*GetVerificationEmailResponse) error
}

type drpcAnyPaymentProcessing_GetVerificationEmailStream struct {
	drpc.Stream
}

func (x *drpcAnyPaymentProcessing_GetVerificationEmailStream) SendAndClose(m *GetVerificationEmailResponse) error {
	if err := x.MsgSend(m, drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{}); err != nil {
		return err
	}
	return x.CloseSend()
}

type DRPCAnyPaymentProcessing_VerifyEmailStream interface {
	drpc.Stream
	SendAndClose(*VerifyEmailResponse) error
}

type drpcAnyPaymentProcessing_VerifyEmailStream struct {
	drpc.Stream
}

func (x *drpcAnyPaymentProcessing_VerifyEmailStream) SendAndClose(m *VerifyEmailResponse) error {
	if err := x.MsgSend(m, drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{}); err != nil {
		return err
	}
	return x.CloseSend()
}

type DRPCAnyPaymentProcessing_GetAllTiersStream interface {
	drpc.Stream
	SendAndClose(*GetTiersResponse) error
}

type drpcAnyPaymentProcessing_GetAllTiersStream struct {
	drpc.Stream
}

func (x *drpcAnyPaymentProcessing_GetAllTiersStream) SendAndClose(m *GetTiersResponse) error {
	if err := x.MsgSend(m, drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{}); err != nil {
		return err
	}
	return x.CloseSend()
}

type DRPCAnyPaymentProcessing_VerifyAppStoreReceiptStream interface {
	drpc.Stream
	SendAndClose(*VerifyAppStoreReceiptResponse) error
}

type drpcAnyPaymentProcessing_VerifyAppStoreReceiptStream struct {
	drpc.Stream
}

func (x *drpcAnyPaymentProcessing_VerifyAppStoreReceiptStream) SendAndClose(m *VerifyAppStoreReceiptResponse) error {
	if err := x.MsgSend(m, drpcEncoding_File_paymentservice_paymentserviceproto_protos_paymentservice_proto{}); err != nil {
		return err
	}
	return x.CloseSend()
}

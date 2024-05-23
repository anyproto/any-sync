package paymentserviceproto

import (
	"errors"

	"github.com/anyproto/any-sync/net/rpc/rpcerr"
)

var (
	errGroup = rpcerr.ErrGroup(ErrorCodes_ErrorOffset)

	ErrEthAddressEmpty           = errGroup.Register(errors.New("owner eth address is empty"), uint64(ErrorCodes_EthAddressEmpty))
	ErrInvalidSignature          = errGroup.Register(errors.New("invalid signature"), uint64(ErrorCodes_InvalidSignature))
	ErrTierWrong                 = errGroup.Register(errors.New("wrong tier specified"), uint64(ErrorCodes_TierWrong))
	ErrTierNotFound              = errGroup.Register(errors.New("requested tier not found"), uint64(ErrorCodes_TierNotFound))
	ErrTierInactive              = errGroup.Register(errors.New("requested tier is not active"), uint64(ErrorCodes_TierInactive))
	ErrPaymentMethodWrong        = errGroup.Register(errors.New("wrong payment method specified"), uint64(ErrorCodes_PaymentMethodWrong))
	ErrBadAnyName                = errGroup.Register(errors.New("bad any name specified"), uint64(ErrorCodes_BadAnyName))
	ErrUnknown                   = errGroup.Register(errors.New("unknown error"), uint64(ErrorCodes_Unknown))
	ErrSubsAlreadyActive         = errGroup.Register(errors.New("user already has an active subscription"), uint64(ErrorCodes_SubsAlreadyActive))
	ErrSubsNotFound              = errGroup.Register(errors.New("no subscription for user"), uint64(ErrorCodes_SubsNotFound))
	ErrSubsWrongState            = errGroup.Register(errors.New("subscription is not in PendingRequiresFinalization state"), uint64(ErrorCodes_SubsWrongState))
	ErrEmailWrongFormat          = errGroup.Register(errors.New("wrong email format"), uint64(ErrorCodes_EmailWrongFormat))
	ErrEmailAlreadyVerified      = errGroup.Register(errors.New("email already verified"), uint64(ErrorCodes_EmailAlreadyVerified))
	ErrEmailAlreadySent          = errGroup.Register(errors.New("email verification request already sent. wait for the email or try again later"), uint64(ErrorCodes_EmailAlreadySent))
	ErrEmailFailedToSend         = errGroup.Register(errors.New("failed to send email"), uint64(ErrorCodes_EmailFailedToSend))
	ErrEmailExpired              = errGroup.Register(errors.New("email verification request expired. try getting new code"), uint64(ErrorCodes_EmailExpired))
	ErrEmailWrongCode            = errGroup.Register(errors.New("wrong verification code"), uint64(ErrorCodes_EmailWrongCode))
	ErrAppleInvalidReceipt       = errGroup.Register(errors.New("invalid AppStore receipt"), uint64(ErrorCodes_AppleInvalidReceipt))
	ErrApplePurchaseRegistration = errGroup.Register(errors.New("error on purchase registration"), uint64(ErrorCodes_ApplePurchaseRegistration))
	ErrAppleSubscriptionRenew    = errGroup.Register(errors.New("error on subscription renew"), uint64(ErrorCodes_AppleSubscriptionRenew))
)

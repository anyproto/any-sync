package pubsubproto

import (
	"errors"

	"github.com/anyproto/any-sync/net/rpc/rpcerr"
)

var (
	errGroup = rpcerr.ErrGroup(ErrCodes_ErrorOffset)

	ErrUnexpected     = errGroup.Register(errors.New("unexpected error"), uint64(ErrCodes_Unexpected))
	ErrNotAMember     = errGroup.Register(errors.New("identity is not a space member"), uint64(ErrCodes_NotAMember))
	ErrNotResponsible = errGroup.Register(errors.New("peer is not responsible for space"), uint64(ErrCodes_NotResponsible))
	ErrRateLimited    = errGroup.Register(errors.New("publish rate limit exceeded"), uint64(ErrCodes_RateLimited))
	ErrTooManyTopics  = errGroup.Register(errors.New("too many topic patterns"), uint64(ErrCodes_TooManyTopics))
	ErrInvalidMessage = errGroup.Register(errors.New("invalid message"), uint64(ErrCodes_InvalidMessage))
	ErrTopicNotOwned  = errGroup.Register(errors.New("topic is owned by another account"), uint64(ErrCodes_TopicNotOwned))
	ErrInvalidTopic   = errGroup.Register(errors.New("invalid topic or pattern"), uint64(ErrCodes_InvalidTopic))
)

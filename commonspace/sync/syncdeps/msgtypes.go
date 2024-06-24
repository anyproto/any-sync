package syncdeps

const (
	MsgTypeIncoming = iota
	MsgTypeOutgoing
	MsgTypeIncomingRequest
	MsgTypeOutgoingRequest
	MsgTypeReceivedResponse
	MsgTypeSentResponse
)

package pool

import "context"

// 1. message for one peerId with ack
// 		pool.SendAndWait(ctx context,.C
// 2. message for many peers without ack (or group)

type Request struct {
	groupId   string
	oneOf     []string
	all       []string
	tryDial   bool
	needReply bool
	pool      *pool
}

func (r *Request) GroupId(groupId string) *Request {
	r.groupId = groupId
	return r
}

func (r *Request) All(peerIds ...string) *Request {
	r.all = peerIds
	return r
}

func (r *Request) OneOf(peerIds ...string) *Request {
	r.oneOf = peerIds
	return r
}

func (r *Request) TryDial(is bool) *Request {
	r.tryDial = is
	return r
}

func (r *Request) NeedReply(is bool) *Request {
	r.needReply = is
	return r
}

func (r *Request) Exec(ctx context.Context, msg *Message) *Results {
	return nil
}

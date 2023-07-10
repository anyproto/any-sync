package syncacl

import (
	"context"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type syncAclHandler struct {
	acl list.AclList
}

func (s *syncAclHandler) HandleMessage(ctx context.Context, senderId string, req *spacesyncproto.ObjectSyncMessage) (err error) {
	return nil
}

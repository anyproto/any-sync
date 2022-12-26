package syncacl

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
)

type syncAclHandler struct {
	acl list.AclList
}

func (s *syncAclHandler) HandleMessage(ctx context.Context, senderId string, req *spacesyncproto.ObjectSyncMessage) (err error) {
	aclMsg := &aclrecordproto.AclSyncMessage{}
	if err = aclMsg.Unmarshal(req.Payload); err != nil {
		return
	}
	content := aclMsg.GetContent()
	switch {
	case content.GetAddRecords() != nil:
		return s.handleAddRecords(ctx, senderId, content.GetAddRecords())
	default:
		return fmt.Errorf("unexpected aclSync message: %T", content.Value)
	}
}

func (s *syncAclHandler) handleAddRecords(ctx context.Context, senderId string, addRecord *aclrecordproto.AclAddRecords) (err error) {
	return
}

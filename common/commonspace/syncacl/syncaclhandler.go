package syncacl

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/list"
)

type syncAclHandler struct {
	acl list.ACLList
}

func (s *syncAclHandler) HandleMessage(ctx context.Context, senderId string, req *spacesyncproto.ObjectSyncMessage) (err error) {
	aclMsg := &aclrecordproto.ACLSyncMessage{}
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

func (s *syncAclHandler) handleAddRecords(ctx context.Context, senderId string, addRecord *aclrecordproto.ACLAddRecords) (err error) {
	return
}

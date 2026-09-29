package list

import (
	"errors"

	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"

	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
)

// ErrEmptyRecordId refuses a record without an id: what it creates would be keyed by "".
var ErrEmptyRecordId = errors.New("acl record has no id")

// withResolvedSelfReferences returns the record with every empty request or invite reference in its content
// replaced by the record's own id. An empty reference can only name the record carrying it: admission used
// to apply a record before it had an id, keying what the record created by "", so a later value in the same
// record reached it that way. Resolving it here replays such a record exactly as it was admitted, and gives
// a new one the same meaning at admission (under its provisional id, see Unmarshall) and on replay. The
// record is copied only when it has something to resolve.
//
// Any other reference that does not resolve applies as a no-op: validation refuses it, so it is reached
// only without validation, for a record already in the log.
func withResolvedSelfReferences(record *AclRecord) *AclRecord {
	data, ok := record.Model.(*aclrecordproto.AclData)
	if !ok {
		return record
	}
	var resolved []*aclrecordproto.AclContentValue
	for i, content := range data.GetAclContent() {
		if ref := contentReference(content); ref == nil || *ref != "" {
			continue
		}
		if resolved == nil {
			resolved = slices.Clone(data.AclContent)
		}
		content = proto.Clone(content).(*aclrecordproto.AclContentValue)
		*contentReference(content) = record.Id
		resolved[i] = content
	}
	if resolved == nil {
		return record
	}
	withResolved := *record
	withResolved.Model = &aclrecordproto.AclData{AclContent: resolved}
	return &withResolved
}

// contentReference points at the id of the request or invite a content value refers to, or is nil when it
// refers to none.
func contentReference(content *aclrecordproto.AclContentValue) *string {
	switch {
	case content.GetRequestAccept() != nil:
		return &content.GetRequestAccept().RequestRecordId
	case content.GetRequestDecline() != nil:
		return &content.GetRequestDecline().RequestRecordId
	case content.GetRequestCancel() != nil:
		return &content.GetRequestCancel().RecordId
	case content.GetRequestJoin() != nil:
		return &content.GetRequestJoin().InviteRecordId
	case content.GetInviteJoin() != nil:
		return &content.GetInviteJoin().InviteRecordId
	case content.GetInviteRevoke() != nil:
		return &content.GetInviteRevoke().InviteRecordId
	case content.GetInviteChange() != nil:
		return &content.GetInviteChange().InviteRecordId
	}
	return nil
}

package list

import "github.com/anyproto/any-sync/util/crypto"

// ParentAuthority is what the parent space lets an account do in a child space the account need not be
// a member of.
type ParentAuthority struct {
	// RemoveMembers allows removing members of the child with AclAccountRemoveNoRotate
	RemoveMembers bool
	// ChangeOwner allows handing the child to one of its members with AclOwnershipChange
	ChangeOwner bool
}

// ParentAuthorizer tells what the parent space lets the author of a record do. It is asked when a record
// is validated before it enters the log. Without an authorizer nobody has any authority.
//
// The acl of a child stores nothing about its parent, so nothing can be asked about a record that is
// already accepted. A list applies such a record as it is when it is built from stored records, and
// when it ingests one under a verifier that does not validate content or that vouches for the accepted
// log (recordverifier.NewValidateAcceptedLog). Any other list refuses to ingest it.
type ParentAuthorizer func(author crypto.PubKey) (ParentAuthority, error)

// grantParentAuthority answers every author with the same authority
func grantParentAuthority(authority ParentAuthority) ParentAuthorizer {
	return func(crypto.PubKey) (ParentAuthority, error) {
		return authority, nil
	}
}

// acceptedLogVerifier is the verifier of a list that ingests only records the network accepted
type acceptedLogVerifier interface {
	AcceptedLog() bool
}

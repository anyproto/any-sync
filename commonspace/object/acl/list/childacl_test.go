package list

import (
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/listtest"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/crypto"
)

var (
	authorityRemove      = ParentAuthority{RemoveMembers: true}
	authorityChangeOwner = ParentAuthority{ChangeOwner: true}
	authorityAll         = ParentAuthority{RemoveMembers: true, ChangeOwner: true}
)

// childAcl is the acl log of a child space as the network keeps it. The coordinator admits a record on
// its own copy of the log, the network signs what it admits, and every account builds its state from the
// signed log. "owner" owns the child, "parent" is an account of the parent space that is not a member of
// the child, "outsider" was never a member, and "coordinator" is the account that admits records.
type childAcl struct {
	netKey crypto.PrivKey
	keys   map[string]*accountdata.AccountKeys
	log    []*consensusproto.RawRecordWithId
}

func newChildAcl(t *testing.T, members map[string]AclPermissions) *childAcl {
	t.Helper()
	netKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	c := &childAcl{netKey: netKey, keys: map[string]*accountdata.AccountKeys{}}
	for _, name := range []string{"owner", "parent", "outsider", "coordinator"} {
		c.newAccount(t, name)
	}
	root, err := buildDerivedRoot("childSpaceId", c.keys["owner"], []byte("metadata"))
	require.NoError(t, err)
	c.log = []*consensusproto.RawRecordWithId{root}
	if len(members) > 0 {
		c.addMembers(t, "owner", members)
	}
	return c
}

func (c *childAcl) newAccount(t *testing.T, name string) crypto.PubKey {
	keys, err := accountdata.NewRandom()
	require.NoError(t, err)
	c.keys[name] = keys
	return keys.SignKey.GetPublic()
}

func (c *childAcl) pub(name string) crypto.PubKey {
	return c.keys[name].SignKey.GetPublic()
}

func (c *childAcl) pubs(names ...string) []crypto.PubKey {
	res := make([]crypto.PubKey, 0, len(names))
	for _, name := range names {
		res = append(res, c.pub(name))
	}
	return res
}

func (c *childAcl) head() string {
	return c.log[len(c.log)-1].Id
}

func (c *childAcl) networkVerifier() recordverifier.AcceptorVerifier {
	return recordverifier.New(c.netKey.GetPublic())
}

// build builds the acl of the account from the signed log
func (c *childAcl) build(t *testing.T, name string, verifier recordverifier.AcceptorVerifier) AclList {
	t.Helper()
	storage, err := NewInMemoryStorage(c.log[0].Id, c.log)
	require.NoError(t, err)
	l, err := BuildAclListWithIdentity(c.keys[name], storage, verifier)
	require.NoError(t, err)
	return l
}

// state is the state the account builds as a client of the network
func (c *childAcl) state(t *testing.T, name string) *AclState {
	t.Helper()
	return c.build(t, name, c.networkVerifier()).AclState()
}

// builder builds records as the account does, on the log the network serves
func (c *childAcl) builder(t *testing.T, name string) AclRecordBuilder {
	t.Helper()
	return c.build(t, name, c.networkVerifier()).RecordBuilder()
}

// admit validates the record as the coordinator does before the network accepts it
func (c *childAcl) admit(t *testing.T, rec *consensusproto.RawRecord, authorizer ParentAuthorizer) error {
	t.Helper()
	return c.build(t, "coordinator", c.networkVerifier()).ValidateRawRecordWithAuthorizer(rec, authorizer, nil)
}

// add admits the record and appends it to the log
func (c *childAcl) add(t *testing.T, rec *consensusproto.RawRecord, authorizer ParentAuthorizer) string {
	t.Helper()
	require.NoError(t, c.admit(t, rec, authorizer))
	return c.accept(t, rec)
}

// accept appends the record to the log with the network's signature
func (c *childAcl) accept(t *testing.T, rec *consensusproto.RawRecord) string {
	t.Helper()
	identity, err := c.netKey.GetPublic().Marshall()
	require.NoError(t, err)
	signature, err := c.netKey.Sign(rec.Payload)
	require.NoError(t, err)
	accepted := listtest.WrapAclRecord(&consensusproto.RawRecord{
		Payload:           rec.Payload,
		Signature:         rec.Signature,
		AcceptorIdentity:  identity,
		AcceptorSignature: signature,
	})
	c.log = append(c.log, accepted)
	return accepted.Id
}

// grant is an authorizer of the coordinator: it gives the authority to the named account only
func (c *childAcl) grant(name string, authority ParentAuthority) ParentAuthorizer {
	return func(author crypto.PubKey) (ParentAuthority, error) {
		if author.Equals(c.pub(name)) {
			return authority, nil
		}
		return ParentAuthority{}, nil
	}
}

// signed is a record of author with the given content on the head of the log, made without the checks
// of the record builder
func (c *childAcl) signed(t *testing.T, author string, content ...*aclrecordproto.AclContentValue) *consensusproto.RawRecord {
	t.Helper()
	data, err := (&aclrecordproto.AclData{AclContent: content}).MarshalVT()
	require.NoError(t, err)
	raw, _ := signAclRecord(t, c.keys[author], c.head(), data)
	return raw
}

// resigned is the content of rec signed by another author
func (c *childAcl) resigned(t *testing.T, rec *consensusproto.RawRecord, author string) *consensusproto.RawRecord {
	t.Helper()
	var payload consensusproto.Record
	require.NoError(t, payload.UnmarshalVT(rec.Payload))
	raw, _ := signAclRecord(t, c.keys[author], payload.PrevId, payload.Data)
	return raw
}

func (c *childAcl) removeNoRotateContent(t *testing.T, names ...string) *aclrecordproto.AclContentValue {
	t.Helper()
	var identities [][]byte
	for _, pk := range c.pubs(names...) {
		identity, err := pk.Marshall()
		require.NoError(t, err)
		identities = append(identities, identity)
	}
	return &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_AccountRemoveNoRotate{
		AccountRemoveNoRotate: &aclrecordproto.AclAccountRemoveNoRotate{Identities: identities},
	}}
}

func (c *childAcl) ownershipChangeContent(t *testing.T, newOwner string, kept AclPermissions) *aclrecordproto.AclContentValue {
	t.Helper()
	identity, err := c.pub(newOwner).Marshall()
	require.NoError(t, err)
	return &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_OwnershipChange{
		OwnershipChange: &aclrecordproto.AclOwnershipChange{
			NewOwnerIdentity:    identity,
			OldOwnerPermissions: aclrecordproto.AclUserPermissions(kept),
		},
	}}
}

// addMembers adds new accounts to the child with a record of author
func (c *childAcl) addMembers(t *testing.T, author string, members map[string]AclPermissions) string {
	t.Helper()
	var adds []AccountAdd
	for _, name := range slices.Sorted(maps.Keys(members)) {
		adds = append(adds, AccountAdd{Identity: c.newAccount(t, name), Permissions: members[name], Metadata: mockMetadata})
	}
	rec, err := c.builder(t, author).BuildAccountsAdd(AccountsAddPayload{Additions: adds})
	require.NoError(t, err)
	return c.add(t, rec, nil)
}

// removeNoRotate removes the accounts with the keyless removal "parent" writes on its authority
func (c *childAcl) removeNoRotate(t *testing.T, names ...string) string {
	t.Helper()
	rec, err := c.builder(t, "parent").BuildAccountRemoveNoRotate(c.pubs(names...))
	require.NoError(t, err)
	return c.add(t, rec, c.grant("parent", authorityRemove))
}

// changeOwnerByParent hands the child to newOwner with the record "parent" writes on its authority
func (c *childAcl) changeOwnerByParent(t *testing.T, newOwner string, kept AclPermissions) string {
	t.Helper()
	rec, err := c.builder(t, "parent").BuildOwnershipChangeByParent(OwnershipChangePayload{
		NewOwner:            c.pub(newOwner),
		OldOwnerPermissions: kept,
	})
	require.NoError(t, err)
	return c.add(t, rec, c.grant("parent", authorityChangeOwner))
}

// rotate changes the read key with a record of author
func (c *childAcl) rotate(t *testing.T, author string) string {
	t.Helper()
	rec, err := c.builder(t, author).BuildReadKeyChange(newTestReadKeyChangePayload())
	require.NoError(t, err)
	return c.add(t, rec, nil)
}

// requireReadKey checks whether the account can decrypt the read key of the record
func (c *childAcl) requireReadKey(t *testing.T, name, recordId string, has bool) {
	t.Helper()
	keys, ok := c.state(t, name).Keys()[recordId]
	require.True(t, ok, "%s: no keys of the record", name)
	if has {
		require.NotNil(t, keys.ReadKey, "%s cannot decrypt the read key", name)
	} else {
		require.Nil(t, keys.ReadKey, "%s decrypts the read key", name)
	}
}

func (c *childAcl) status(st *AclState, name string) AclStatus {
	return st.accountStates[mapKeyFromPubKey(c.pub(name))].Status
}

// requireNoAccount checks that the acl keeps no entry for the account
func (c *childAcl) requireNoAccount(t *testing.T, st *AclState, name string) {
	t.Helper()
	for _, acc := range st.CurrentAccounts() {
		require.False(t, acc.PubKey.Equals(c.pub(name)), "the acl keeps an entry for %s", name)
	}
}

func (c *childAcl) requireOwner(t *testing.T, st *AclState, name string) {
	t.Helper()
	owner, err := st.OwnerPubKey()
	require.NoError(t, err)
	require.True(t, owner.Equals(c.pub(name)), "the owner is not %s", name)
}

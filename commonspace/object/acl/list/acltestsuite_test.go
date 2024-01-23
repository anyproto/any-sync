package list

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/crypto"
)

type testAclState struct {
	keys *accountdata.AccountKeys
	acl  AclList
}

type accountExpectedState struct {
	perms    AclPermissions
	status   AclStatus
	metadata []byte
}

type aclExecutor struct {
	owner            string
	spaceId          string
	invites          map[string]crypto.PrivKey
	actualAccounts   map[string]*testAclState
	expectedAccounts map[string]*accountExpectedState
}

func newAclExecutor(spaceId string) *aclExecutor {
	return &aclExecutor{
		spaceId:          spaceId,
		invites:          map[string]crypto.PrivKey{},
		actualAccounts:   make(map[string]*testAclState),
		expectedAccounts: make(map[string]*accountExpectedState),
	}
}

var (
	errIncorrectParts = errors.New("incorrect parts")
)

func (a *aclExecutor) execute(cmd string) error {
	parts := strings.Split(cmd, ":")
	if len(parts) != 2 {
		return errIncorrectParts
	}
	commandParts := strings.Split(parts[0], ".")
	if len(commandParts) != 2 {
		return errIncorrectParts
	}
	account := commandParts[0]
	command := commandParts[1]
	args := strings.Split(parts[1], ";")
	if len(args) == 0 {
		return errIncorrectParts
	}
	if _, exists := a.actualAccounts[account]; !exists {
		keys, err := accountdata.NewRandom()
		if err != nil {
			return err
		}
		if len(a.expectedAccounts) == 0 {
			acl, err := NewTestDerivedAclMetadata(a.spaceId, keys, []byte(account))
			if err != nil {
				return err
			}
			state := &testAclState{
				keys: keys,
				acl:  acl,
			}
			a.actualAccounts[account] = state
			a.expectedAccounts[account] = &accountExpectedState{
				perms:    AclPermissions(aclrecordproto.AclUserPermissions_Owner),
				status:   StatusActive,
				metadata: []byte(account),
			}
			a.owner = account
			return nil
		} else {
			ownerAcl := a.actualAccounts[a.owner].acl.(*aclList)
			accountAcl, err := BuildAclListWithIdentity(keys, ownerAcl.storage, NoOpAcceptorVerifier{})
			if err != nil {
				return err
			}
			state := &testAclState{
				keys: keys,
				acl:  accountAcl,
			}
			a.actualAccounts[account] = state
			a.expectedAccounts[account] = &accountExpectedState{
				metadata: []byte(account),
			}
		}
	} else if a.expectedAccounts[account].status == StatusRemoved {
		keys := a.actualAccounts[account].keys
		ownerAcl := a.actualAccounts[a.owner].acl.(*aclList)
		accountAcl, err := BuildAclListWithIdentity(keys, ownerAcl.storage, NoOpAcceptorVerifier{})
		if err != nil {
			return err
		}
		a.actualAccounts[account].acl = accountAcl
		return nil
	}
	acl := a.actualAccounts[account].acl
	addRec := func(rec *consensusproto.RawRecordWithId) error {
		for _, acc := range a.actualAccounts {
			err := acc.acl.AddRawRecord(rec)
			if err != nil {
				return err
			}
		}
		return nil
	}
	getPerm := func(perm string) AclPermissions {
		var aclPerm aclrecordproto.AclUserPermissions
		switch perm {
		case "adm":
			aclPerm = aclrecordproto.AclUserPermissions_Admin
		case "rw":
			aclPerm = aclrecordproto.AclUserPermissions_Writer
		case "r":
			aclPerm = aclrecordproto.AclUserPermissions_Reader
		}
		return AclPermissions(aclPerm)
	}
	switch command {
	case "join":
		a.expectedAccounts[account].status = StatusJoining
		invite := a.invites[args[0]]
		requestJoin, err := acl.RecordBuilder().BuildRequestJoin(RequestJoinPayload{
			InviteKey: invite,
			Metadata:  []byte(account),
		})
		if err != nil {
			return err
		}
		err = addRec(WrapAclRecord(requestJoin))
		if err != nil {
			return err
		}
	case "invite":
		res, err := acl.RecordBuilder().BuildInvite()
		if err != nil {
			return err
		}
		a.invites[args[0]] = res.InviteKey
		err = addRec(WrapAclRecord(res.InviteRec))
		if err != nil {
			return err
		}
	case "approve":
		recs, err := acl.AclState().JoinRecords(false)
		if err != nil {
			return err
		}
		argParts := strings.Split(args[0], ",")
		if len(argParts) != 2 {
			return errIncorrectParts
		}
		approved := a.actualAccounts[argParts[0]].keys.SignKey.GetPublic()
		var recId string
		for _, rec := range recs {
			if rec.RequestIdentity.Equals(approved) {
				recId = rec.RecordId
			}
		}
		if recId == "" {
			return fmt.Errorf("no join records for approve")
		}
		perms := getPerm(argParts[1])
		res, err := acl.RecordBuilder().BuildRequestAccept(RequestAcceptPayload{
			RequestRecordId: recId,
			Permissions:     perms,
		})
		if err != nil {
			return err
		}
		err = addRec(WrapAclRecord(res))
		if err != nil {
			return err
		}
		a.expectedAccounts[argParts[0]].status = StatusActive
		a.expectedAccounts[argParts[0]].perms = perms
	case "change":
		argParts := strings.Split(args[0], ",")
		if len(argParts) != 2 {
			return errIncorrectParts
		}
		changed := a.actualAccounts[argParts[0]].keys.SignKey.GetPublic()
		perms := getPerm(argParts[1])
		res, err := acl.RecordBuilder().BuildPermissionChange(PermissionChangePayload{
			Identity:    changed,
			Permissions: perms,
		})
		if err != nil {
			return err
		}
		err = addRec(WrapAclRecord(res))
		if err != nil {
			return err
		}
		a.expectedAccounts[argParts[0]].perms = perms
	case "remove":
		identities := strings.Split(args[0], ",")
		var pubKeys []crypto.PubKey
		for _, id := range identities {
			pk := a.actualAccounts[id].keys.SignKey.GetPublic()
			pubKeys = append(pubKeys, pk)
		}
		priv, _, err := crypto.GenerateRandomEd25519KeyPair()
		if err != nil {
			return err
		}
		sym := crypto.NewAES()
		res, err := acl.RecordBuilder().BuildAccountRemove(AccountRemovePayload{
			Identities: pubKeys,
			Change: ReadKeyChangePayload{
				MetadataKey: priv,
				ReadKey:     sym,
			},
		})
		if err != nil {
			return err
		}
		err = addRec(WrapAclRecord(res))
		if err != nil {
			return err
		}
		for _, id := range identities {
			a.expectedAccounts[id].status = StatusRemoved
			a.expectedAccounts[id].perms = AclPermissionsNone
		}
	default:
		return fmt.Errorf("unexpected")
	}
	return nil
}

func (a *aclExecutor) verify(t *testing.T) {
	for id, exp := range a.expectedAccounts {
		key := a.actualAccounts[id].keys.SignKey.GetPublic()
		for actId, act := range a.actualAccounts {
			if a.expectedAccounts[actId].status == StatusRemoved {
				continue
			}
			state := act.acl.AclState()
			require.Equal(t, exp.status, state.accountStates[mapKeyFromPubKey(key)].Status)
			require.Equal(t, exp.perms, state.accountStates[mapKeyFromPubKey(key)].Permissions)
			metadata, err := act.acl.AclState().GetMetadata(key, true)
			require.NoError(t, err)
			require.Equal(t, exp.metadata, metadata)
		}
	}
}

func TestAclExecutor(t *testing.T) {
	a := newAclExecutor("spaceId")
	cmds := []string{
		"a.init:a",
		"a.invite:invId",
		"b.join:invId",
		"a.approve:b,rw",
		"c.join:invId",
		"a.approve:c,r",
		"a.remove:c",
	}
	for _, cmd := range cmds {
		err := a.execute(cmd)
		require.NoError(t, err)
	}
	a.verify(t)
}

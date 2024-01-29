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
	pseudoId string
	recCmd   string
}

type aclExecutor struct {
	owner               string
	spaceId             string
	invites             map[string]crypto.PrivKey
	actualAccounts      map[string]*testAclState
	expectedAccounts    map[string]*accountExpectedState
	expectedPermissions map[string][]accountExpectedState
}

func newAclExecutor(spaceId string) *aclExecutor {
	return &aclExecutor{
		spaceId:             spaceId,
		invites:             map[string]crypto.PrivKey{},
		actualAccounts:      make(map[string]*testAclState),
		expectedAccounts:    make(map[string]*accountExpectedState),
		expectedPermissions: make(map[string][]accountExpectedState),
	}
}

var (
	errIncorrectParts = errors.New("incorrect parts")
)

func (a *aclExecutor) execute(cmd string) (err error) {
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
	var isNewAccount bool
	if _, exists := a.actualAccounts[account]; !exists {
		isNewAccount = true
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
				pseudoId: account,
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
				pseudoId: account,
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
	}
	acl := a.actualAccounts[account].acl
	var afterAll []func()
	defer func() {
		if err != nil {
			if !isNewAccount {
				return
			}
			delete(a.expectedAccounts, account)
			delete(a.actualAccounts, account)
		} else {
			for _, f := range afterAll {
				f()
			}
			head := acl.Head().Id
			var states []accountExpectedState
			for _, state := range a.expectedAccounts {
				cp := *state
				cp.recCmd = cmd
				states = append(states, cp)
			}
			a.expectedPermissions[head] = states
		}
	}()
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
		a.expectedAccounts[account].status = StatusJoining
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
	case "changes":
		var payloads []PermissionChangePayload
		for _, arg := range args {
			argParts := strings.Split(arg, ",")
			if len(argParts) != 2 {
				return errIncorrectParts
			}
			changed := a.actualAccounts[argParts[0]].keys.SignKey.GetPublic()
			perms := getPerm(argParts[1])
			payloads = append(payloads, PermissionChangePayload{
				Identity:    changed,
				Permissions: perms,
			})
			afterAll = append(afterAll, func() {
				a.expectedAccounts[argParts[0]].perms = perms
			})
		}
		permissionChanges := PermissionChangesPayload{Changes: payloads}
		res, err := acl.RecordBuilder().BuildPermissionChanges(permissionChanges)
		if err != nil {
			return err
		}
		err = addRec(WrapAclRecord(res))
		if err != nil {
			return err
		}
	case "add":
		var payloads []AccountAdd
		for _, arg := range args {
			argParts := strings.Split(arg, ",")
			if len(argParts) != 3 {
				return errIncorrectParts
			}
			keys, err := accountdata.NewRandom()
			if err != nil {
				return err
			}
			ownerAcl := a.actualAccounts[a.owner].acl.(*aclList)
			accountAcl, err := BuildAclListWithIdentity(keys, ownerAcl.storage, NoOpAcceptorVerifier{})
			if err != nil {
				return err
			}
			state := &testAclState{
				keys: keys,
				acl:  accountAcl,
			}
			account = argParts[0]
			a.actualAccounts[account] = state
			a.expectedAccounts[account] = &accountExpectedState{
				perms:    getPerm(argParts[1]),
				status:   StatusActive,
				metadata: []byte(argParts[2]),
				pseudoId: account,
			}
			payloads = append(payloads, AccountAdd{
				Identity:    keys.SignKey.GetPublic(),
				Permissions: getPerm(argParts[1]),
				Metadata:    []byte(argParts[2]),
			})
		}
		defer func() {
			if err != nil {
				for _, arg := range args {
					argParts := strings.Split(arg, ",")
					account := argParts[0]
					delete(a.expectedAccounts, account)
					delete(a.actualAccounts, account)
				}
			}
		}()
		res, err := acl.RecordBuilder().BuildAccountsAdd(AccountsAddPayload{Additions: payloads})
		if err != nil {
			return err
		}
		err = addRec(WrapAclRecord(res))
		if err != nil {
			return err
		}
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
	case "request_remove":
		id := args[0]
		res, err := acl.RecordBuilder().BuildRequestRemove()
		if err != nil {
			return err
		}
		err = addRec(WrapAclRecord(res))
		if err != nil {
			return err
		}
		a.expectedAccounts[id].status = StatusRemoving
	case "decline":
		id := args[0]
		pk := a.actualAccounts[id].keys.SignKey.GetPublic()
		rec, err := acl.AclState().JoinRecord(pk, false)
		if err != nil {
			return err
		}
		res, err := acl.RecordBuilder().BuildRequestDecline(rec.RecordId)
		if err != nil {
			return err
		}
		err = addRec(WrapAclRecord(res))
		if err != nil {
			return err
		}
		a.expectedAccounts[id].status = StatusDeclined
	case "cancel":
		id := args[0]
		pk := a.actualAccounts[id].keys.SignKey.GetPublic()
		rec, err := acl.AclState().Record(pk)
		if err != nil {
			return err
		}
		res, err := acl.RecordBuilder().BuildRequestCancel(rec.RecordId)
		if err != nil {
			return err
		}
		err = addRec(WrapAclRecord(res))
		if err != nil {
			return err
		}
		if rec.Type == RequestTypeJoin {
			a.expectedAccounts[id].status = StatusCanceled
		} else {
			a.expectedAccounts[id].status = StatusActive
		}
	case "revoke":
		invite := a.invites[args[0]]
		invId, err := acl.AclState().GetInviteIdByPrivKey(invite)
		if err != nil {
			return err
		}
		requestJoin, err := acl.RecordBuilder().BuildInviteRevoke(invId)
		if err != nil {
			return err
		}
		err = addRec(WrapAclRecord(requestJoin))
		if err != nil {
			return err
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
			if a.expectedAccounts[actId].status != StatusActive {
				metadata, err := act.acl.AclState().GetMetadata(key, false)
				require.NoError(t, err)
				require.NotNil(t, metadata)
				continue
			}
			metadata, err := act.acl.AclState().GetMetadata(key, true)
			require.NoError(t, err)
			require.Equal(t, exp.metadata, metadata)
		}
	}
	for aclRecId, perms := range a.expectedPermissions {
		for _, perm := range perms {
			for actId, acc := range a.actualAccounts {
				if a.expectedAccounts[actId].status == StatusRemoved {
					continue
				}
				identity := a.actualAccounts[perm.pseudoId].keys.SignKey.GetPublic()
				actualPerms, err := acc.acl.AclState().PermissionsAtRecord(aclRecId, identity)
				require.NoError(t, err, perm.recCmd)
				require.Equal(t, perm.perms, actualPerms, fmt.Sprintf("%s, %s, %s", perm.recCmd, perm.pseudoId, actId))
			}
		}
	}
}

func TestAclExecutor(t *testing.T) {
	a := newAclExecutor("spaceId")
	type cmdErr struct {
		cmd string
		err error
	}
	cmds := []cmdErr{
		{"a.init:a", nil},
		// creating an invite
		{"a.invite:invId", nil},
		// cannot self join
		{"a.join:invId", ErrInsufficientPermissions},
		// now b can join
		{"b.join:invId", nil},
		// a approves b, it can write now
		{"a.approve:b,r", nil},
		// c joins with the same invite
		{"c.join:invId", nil},
		// a approves c
		{"a.approve:c,r", nil},
		// a removes c
		{"a.remove:c", nil},
		// e also joins as an admin
		{"e.join:invId", nil},
		{"a.approve:e,adm", nil},
		// now e can remove other users
		{"e.remove:b", nil},
		{"e.revoke:invId", nil},
		{"z.join:invId", ErrNoSuchInvite},
		// e can't revoke the same id
		{"e.revoke:invId", ErrNoSuchRecord},
		// e can't remove a, because a is the owner
		{"e.remove:a", ErrInsufficientPermissions},
		// e can add new users
		{"e.add:x,r,m1;y,adm,m2", nil},
		// now y can also change permission as an admin
		{"y.changes:x,rw", nil},
		// e can generate another invite
		{"e.invite:inv1Id", nil},
		// b tries to join again
		{"b.join:inv1Id", nil},
		// e approves b
		{"e.approve:b,rw", nil},
	}
	for _, cmd := range cmds {
		err := a.execute(cmd.cmd)
		require.Equal(t, cmd.err, err, cmd)
	}
	a.verify(t)
}

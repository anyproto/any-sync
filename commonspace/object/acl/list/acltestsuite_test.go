package list

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func (a *AclTestExecutor) verify(t *testing.T) {
	for id, exp := range a.expectedAccounts {
		key := a.actualAccounts[id].Keys.SignKey.GetPublic()
		for actId, act := range a.actualAccounts {
			if a.expectedAccounts[actId].status == StatusRemoved {
				continue
			}
			state := act.Acl.AclState()
			require.Equal(t, exp.status, state.accountStates[mapKeyFromPubKey(key)].Status)
			require.Equal(t, exp.perms, state.accountStates[mapKeyFromPubKey(key)].Permissions)
			if a.expectedAccounts[actId].status != StatusActive {
				metadata, err := act.Acl.AclState().GetMetadata(key, false)
				require.NoError(t, err)
				require.NotNil(t, metadata)
				continue
			}
			metadata, err := act.Acl.AclState().GetMetadata(key, true)
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
				identity := a.actualAccounts[perm.pseudoId].Keys.SignKey.GetPublic()
				actualPerms, err := acc.Acl.AclState().PermissionsAtRecord(aclRecId, identity)
				require.NoError(t, err, perm.recCmd)
				require.Equal(t, perm.perms, actualPerms, fmt.Sprintf("%s, %s, %s", perm.recCmd, perm.pseudoId, actId))
			}
		}
	}
}

func TestAclExecutor(t *testing.T) {
	a := NewAclExecutor("spaceId")
	type cmdErr struct {
		cmd string
		err error
	}
	cmds := []cmdErr{
		{"a.init::a", nil},
		// creating an invite
		{"a.invite::invId", nil},
		// cannot self join
		{"a.join::invId", ErrInsufficientPermissions},
		// now b can join
		{"b.join::invId", nil},
		// a approves b, it can write now
		{"a.approve::b,r", nil},
		// c joins with the same invite
		{"c.join::invId", nil},
		// a approves c
		{"a.approve::c,r", nil},
		// a removes c
		{"a.remove::c", nil},
		// e also joins as an admin
		{"e.join::invId", nil},
		{"a.approve::e,adm", nil},
		// now e can remove other users
		{"e.remove::b", nil},
		{"e.revoke::invId", nil},
		{"z.join::invId", ErrNoSuchInvite},
		// e can't revoke the same id
		{"e.revoke::invId", ErrNoSuchRecord},
		// e can't remove a, because a is the owner
		{"e.remove::a", ErrInsufficientPermissions},
		// e can add new users
		{"e.add::x,r,m1;y,adm,m2", nil},
		// now y can also change permission as an admin
		{"y.changes::x,rw", nil},
		// e can generate another invite
		{"e.invite::inv1Id", nil},
		// b tries to join again
		{"b.join::inv1Id", nil},
		// e approves b
		{"e.approve::b,rw", nil},
		{"g.join::inv1Id", nil},
		{"g.cancel::g", nil},
		// e cannot approve cancelled request
		{"e.approve::g,rw", fmt.Errorf("no join records for approve")},
		{"g.join::inv1Id", nil},
		{"e.decline::g", nil},
		// g cannot cancel declined request
		{"g.cancel::g", ErrNoSuchRecord},
		{"g.join::inv1Id", nil},
		{"e.approve::g,r", nil},
		// g can request remove
		{"g.request_remove::g", nil},
		// g can cancel request to remove
		{"g.cancel::g", nil},
		{"g.request_remove::g", nil},
		{"g.request_remove::g", ErrPendingRequest},
		{"a.remove::g", nil},
		// g cannot cancel not existing request to remove
		{"g.cancel::g", ErrNoSuchRecord},
		{"a.batch::remove:e,y;add:z,rw,mz|u,r,mu;revoke:inv1Id", nil},
	}
	for _, cmd := range cmds {
		err := a.Execute(cmd.cmd)
		require.Equal(t, cmd.err, err, cmd)
	}
	a.verify(t)
}

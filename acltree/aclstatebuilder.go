package acltree

import (
	"fmt"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/aclchanges/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
)

type ACLStateBuilder struct {
	tree     *Tree
	aclState *aclState
	identity string
	key      keys.EncryptionPrivKey
	decoder  keys.SigningPubKeyDecoder
}

type decreasedPermissionsParameters struct {
	users       []*pb.ACLChangeUserPermissionChange
	startChange string
}

func NewACLStateBuilder(decoder keys.SigningPubKeyDecoder, accountData *account.AccountData) *ACLStateBuilder {
	return &ACLStateBuilder{
		decoder:  decoder,
		identity: accountData.Identity,
		key:      accountData.EncKey,
	}
}

func (sb *ACLStateBuilder) Build() (*aclState, error) {
	state, _, err := sb.BuildBefore("")
	return state, err
}

func (sb *ACLStateBuilder) Init(tree *Tree) error {
	root := tree.Root()
	if !root.IsSnapshot {
		return fmt.Errorf("root should always be a snapshot")
	}

	snapshot := root.Content.GetAclData().GetAclSnapshot()
	state, err := NewACLStateFromSnapshot(
		snapshot,
		sb.identity,
		sb.key,
		sb.decoder)
	if err != nil {
		return fmt.Errorf("could not build aclState from snapshot: %w", err)
	}
	sb.tree = tree
	sb.aclState = state

	return nil
}

// TODO: we can probably have only one state builder, because we can build both at the same time
func (sb *ACLStateBuilder) BuildBefore(beforeId string) (*aclState, bool, error) {
	var (
		err                  error
		startChange          = sb.tree.root
		foundId              bool
		idSeenMap            = make(map[string][]*Change)
		decreasedPermissions *decreasedPermissionsParameters
	)
	idSeenMap[startChange.Content.Identity] = append(idSeenMap[startChange.Content.Identity], startChange)

	if startChange.Content.GetChangesData() != nil {
		key, exists := sb.aclState.userReadKeys[startChange.Content.CurrentReadKeyHash]
		if !exists {
			return nil, false, fmt.Errorf("no first snapshot")
		}

		err = startChange.DecryptContents(key)
		if err != nil {
			return nil, false, fmt.Errorf("failed to decrypt contents of first snapshot")
		}
	}

	if beforeId == startChange.Id {
		return sb.aclState, true, nil
	}

	for {
		// TODO: we should optimize this method to just remember last state of iterator and not iterate from the start and skip if nothing was removed from the tree
		sb.tree.iterateSkip(sb.tree.root, startChange, func(c *Change) (isContinue bool) {
			defer func() {
				if err == nil {
					startChange = c
				} else if err != ErrDocumentForbidden {
					//log.Errorf("marking change %s as invalid: %v", c.Id, err)
					sb.tree.RemoveInvalidChange(c.Id)
				}
			}()

			// not applying root change
			if c.Id == startChange.Id {
				return true
			}

			idSeenMap[c.Content.Identity] = append(idSeenMap[c.Content.Identity], c)
			if c.Content.GetAclData() != nil {
				err = sb.aclState.ApplyChange(c.Id, c.Content)
				if err != nil {
					return false
				}

				// if we have some users who have less permissions now
				users := sb.aclState.GetPermissionDecreasedUsers(c.Content)
				if len(users) > 0 {
					decreasedPermissions = &decreasedPermissionsParameters{
						users:       users,
						startChange: c.Id,
					}
					return false
				}
			}

			// the user can't make changes
			if !sb.aclState.HasPermission(c.Content.Identity, pb.ACLChange_Writer) && !sb.aclState.HasPermission(c.Content.Identity, pb.ACLChange_Admin) {
				err = fmt.Errorf("user %s cannot make changes", c.Content.Identity)
				return false
			}

			// decrypting contents on the fly
			if c.Content.GetChangesData() != nil {
				key, exists := sb.aclState.userReadKeys[c.Content.CurrentReadKeyHash]
				if !exists {
					err = fmt.Errorf("failed to find key with hash: %d", c.Content.CurrentReadKeyHash)
					return false
				}

				err = c.DecryptContents(key)
				if err != nil {
					err = fmt.Errorf("failed to decrypt contents for hash: %d", c.Content.CurrentReadKeyHash)
					return false
				}
			}

			if c.Id == beforeId {
				foundId = true
				return false
			}

			return true
		})

		// if we have users with decreased permissions
		if decreasedPermissions != nil {
			var removed bool
			validChanges := sb.tree.dfs(decreasedPermissions.startChange)

			for _, permChange := range decreasedPermissions.users {
				seenChanges := idSeenMap[permChange.Identity]

				for _, seen := range seenChanges {
					// if we find some invalid changes
					if _, exists := validChanges[seen.Id]; !exists {
						// if the user didn't have enough permission to make changes
						if seen.IsACLChange() || permChange.Permissions > pb.ACLChange_Writer {
							removed = true
							sb.tree.RemoveInvalidChange(seen.Id)
						}
					}
				}
			}

			decreasedPermissions = nil
			if removed {
				// starting from the beginning but with updated tree
				return sb.BuildBefore(beforeId)
			}
		} else if err == nil {
			// we can finish the acl state building process
			break
		}

		// the user is forbidden to access the document
		if err == ErrDocumentForbidden {
			return nil, foundId, err
		}

		// otherwise we have to continue from the change which we had
		err = nil
	}

	return sb.aclState, foundId, err
}

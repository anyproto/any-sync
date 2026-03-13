package settings

import (
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

func settingsContentValidator(c *objecttree.Change, aclList list.AclList) error {
	if c.Data == nil || len(c.PreviousIds) == 0 {
		return nil // skip root
	}
	settingsData := &spacesyncproto.SettingsData{}
	if err := settingsData.UnmarshalVT(c.Data); err != nil {
		return nil
	}
	hasDelete := false
	for _, cnt := range settingsData.Content {
		if cnt.GetObjectDelete() != nil {
			hasDelete = true
			break
		}
	}
	if !hasDelete {
		return nil
	}
	// Same pattern as objecttreevalidator: resolve state at ACL head
	state := aclList.AclState()
	opts := state.OptionsAtRecord(c.AclHeadId)
	if opts == nil || !opts.DeleteRestricted {
		return nil
	}
	perms, err := state.PermissionsAtRecord(c.AclHeadId, c.Identity)
	if err != nil {
		return err
	}
	if !perms.CanManageAccounts() {
		return list.ErrInsufficientPermissions
	}
	return nil
}

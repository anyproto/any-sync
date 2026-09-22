package settings

import (
	"context"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/util/crypto"
)

// objectAuthor resolves the effective author of an object. For child/derived objects
// (those with a ParentId), it resolves to the parent object's author.
func objectAuthor(store spacestorage.SpaceStorage, objectId string) (crypto.PubKey, error) {
	ctx := context.Background()
	entry, err := store.HeadStorage().GetEntry(ctx, objectId)
	if err != nil {
		return nil, err
	}
	targetId := objectId
	if entry.ParentId != "" {
		targetId = entry.ParentId
	}
	treeStorage, err := store.TreeStorage(ctx, targetId)
	if err != nil {
		return nil, err
	}
	rootChange, err := treeStorage.Root(ctx)
	if err != nil {
		return nil, err
	}
	root, err := objecttree.UnmarshallRoot(&treechangeproto.RawTreeChangeWithId{
		RawChange: rootChange.RawChange,
		Id:        rootChange.Id,
	})
	if err != nil {
		return nil, err
	}
	return crypto.NewKeyStorage().PubKeyFromProto(root.Identity)
}

func newSettingsContentValidator(getAuthor func(string) (crypto.PubKey, error)) func(*objecttree.Change, list.AclList) error {
	return func(c *objecttree.Change, aclList list.AclList) error {
		if c.Data == nil || len(c.PreviousIds) == 0 {
			return nil // skip root
		}
		settingsData := &spacesyncproto.SettingsData{}
		if err := settingsData.UnmarshalVT(c.Data); err != nil {
			return nil
		}
		var deleteIds []string
		for _, cnt := range settingsData.Content {
			if od := cnt.GetObjectDelete(); od != nil {
				deleteIds = append(deleteIds, od.Id)
			}
		}
		if len(deleteIds) == 0 {
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
		if perms.CanManageAccounts() {
			return nil
		}
		// Check if the change author is the author of all objects being deleted
		for _, objId := range deleteIds {
			author, err := getAuthor(objId)
			if err != nil || !author.Equals(c.Identity) {
				return list.ErrInsufficientPermissions
			}
		}
		return nil
	}
}

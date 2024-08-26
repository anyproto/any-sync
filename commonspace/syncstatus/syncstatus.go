package syncstatus

import (
	"github.com/anyproto/any-sync/app"
)

const CName = "common.commonspace.syncstatus"

type StatusUpdater interface {
	app.ComponentRunnable

	HeadsChange(treeId string, heads []string)
	HeadsReceive(senderId, treeId string, heads []string)
	ObjectReceive(senderId, treeId string, heads []string)
	HeadsApply(senderId, treeId string, heads []string, allAdded bool)
}

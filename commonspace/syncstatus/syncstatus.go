//go:generate mockgen -destination mock_syncstatus/mock_syncstatus.go github.com/anyproto/any-sync/commonspace/syncstatus StatusUpdater
package syncstatus

import (
	"github.com/anyproto/any-sync/app"
)

const CName = "common.commonspace.syncstatus"

type StatusUpdater interface {
	app.Component

	HeadsChange(treeId string, heads []string)
	HeadsReceive(senderId, treeId string, heads []string)
	ObjectReceive(senderId, treeId string, heads []string)
	HeadsApply(senderId, treeId string, heads []string, allAdded bool)
}

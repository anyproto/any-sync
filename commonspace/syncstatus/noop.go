package syncstatus

import (
	"context"

	"github.com/anyproto/any-sync/app"
)

func NewNoOpSyncStatus() StatusUpdater {
	return &noOpSyncStatus{}
}

type noOpSyncStatus struct{}

func (n *noOpSyncStatus) Init(a *app.App) (err error) {
	return nil
}

func (n *noOpSyncStatus) Name() (name string) {
	return CName
}

func (n *noOpSyncStatus) HeadsChange(treeId string, heads []string) {
}

func (n *noOpSyncStatus) HeadsApply(senderId, treeId string, heads []string) {
}

func (n *noOpSyncStatus) HeadsReceive(senderId, treeId string, heads []string) {
}

func (n *noOpSyncStatus) StateCounter() uint64 {
	return 0
}

func (n *noOpSyncStatus) Run(ctx context.Context) error {
	return nil
}

func (n *noOpSyncStatus) Close(ctx context.Context) error {
	return nil
}

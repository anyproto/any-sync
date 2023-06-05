package syncstatus

import (
	"context"
	"github.com/anyproto/any-sync/app"
)

func NewNoOpSyncStatus() StatusService {
	return &noOpSyncStatus{}
}

type noOpSyncStatus struct{}

func (n *noOpSyncStatus) Init(a *app.App) (err error) {
	return nil
}

func (n *noOpSyncStatus) Name() (name string) {
	return CName
}

func (n *noOpSyncStatus) Watch(treeId string) (err error) {
	return nil
}

func (n *noOpSyncStatus) Unwatch(treeId string) {
}

func (n *noOpSyncStatus) SetUpdateReceiver(updater UpdateReceiver) {
}

func (n *noOpSyncStatus) HeadsChange(treeId string, heads []string) {
}

func (n *noOpSyncStatus) HeadsReceive(senderId, treeId string, heads []string) {
}

func (n *noOpSyncStatus) SetNodesOnline(senderId string, online bool) {
}

func (n *noOpSyncStatus) StateCounter() uint64 {
	return 0
}

func (n *noOpSyncStatus) RemoveAllExcept(senderId string, differentRemoteIds []string, stateCounter uint64) {
}

func (n *noOpSyncStatus) Run(ctx context.Context) error {
	return nil
}

func (n *noOpSyncStatus) Close(ctx context.Context) error {
	return nil
}

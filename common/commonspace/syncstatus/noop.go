package syncstatus

type noOpSyncStatus struct{}

func NewNoOpSyncStatus() SyncStatusUpdater {
	return &noOpSyncStatus{}
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

func (n *noOpSyncStatus) Run() {
}

func (n *noOpSyncStatus) Close() error {
	return nil
}

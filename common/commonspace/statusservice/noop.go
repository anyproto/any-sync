package statusservice

type noOpStatusService struct{}

func NewNoOpStatusService() StatusService {
	return &noOpStatusService{}
}

func (n *noOpStatusService) HeadsChange(treeId string, heads []string) {
}

func (n *noOpStatusService) HeadsReceive(senderId, treeId string, heads []string) {
}

func (n *noOpStatusService) Watch(treeId string) (err error) {
	return
}

func (n *noOpStatusService) Unwatch(treeId string) {
}

func (n *noOpStatusService) SetNodesOnline(senderId string, online bool) {
}

func (n *noOpStatusService) StateCounter() uint64 {
	return 0
}

func (n *noOpStatusService) RemoveAllExcept(senderId string, differentRemoteIds []string, stateCounter uint64) {
}

func (n *noOpStatusService) SetUpdateReceiver(updater UpdateReceiver) {
}

func (n *noOpStatusService) Run() {
}

func (n *noOpStatusService) Close() error {
	return nil
}

package statusservice

type StatusService interface {
	HeadsChange(treeId string, heads []string)
	HeadsReceive(senderId, treeId string, heads []string)
	Watch(treeId string, ch chan struct{})
	Unwatch(treeId string)
	StateCounter() uint64
	RemoveAllExcept(senderId string, differentRemoteIds []string, stateCounter uint64)
}

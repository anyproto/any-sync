package sync

type RequestPool interface {
	QueueRequestAction(peerId, objectId string, action func()) (err error)
}

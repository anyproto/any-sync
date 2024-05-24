package sync

type SyncDeps struct {
	HeadUpdateHandler HeadUpdateHandler
	HeadUpdateSender  HeadUpdateSender
	ResponseHandler   ResponseHandler
	RequestHandler    RequestHandler
	RequestSender     RequestSender
	MergeFilter       MergeFilterFunc
}

package streampool

type StreamConfig struct {
	// SendQueueSize size of the queue for write per peer
	SendQueueSize int
	// DialQueueWorkers how many workers will dial to peers
	DialQueueWorkers int
	// DialQueueSize size of the dial queue
	DialQueueSize int
}

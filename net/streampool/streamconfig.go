package streampool

import (
	"github.com/anyproto/any-sync/app/logger"
)

const CName = "common.net.streampool"

var log = logger.NewNamed(CName)

type StreamConfig struct {
	// SendQueueSize size of the queue for write per peer
	SendQueueSize int
	// DialQueueWorkers how many workers will dial to peers
	DialQueueWorkers int
	// DialQueueSize size of the dial queue
	DialQueueSize int
}

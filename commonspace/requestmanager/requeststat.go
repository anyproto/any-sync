package requestmanager

import (
	"sync"

	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type requestStat struct {
	sync.Mutex
	peerStats map[string]peerStat
}

func newRequestStat() *requestStat {
	return &requestStat{
		peerStats: make(map[string]peerStat),
	}
}

type spaceQueueStat struct {
	TotalSize int        `json:"total_size"`
	PeerStats []peerStat `json:"peer_stats"`
}

type peerStat struct {
	QueueCount int    `json:"queue_count"`
	QueueSize  int    `json:"queue_size"`
	SyncSize   int    `json:"sync_size"`
	SyncCount  int    `json:"sync_count"`
	PeerId     string `json:"peer_id"`
}

func (r *requestStat) AddQueueRequest(peerId string, size int) {
	r.Lock()
	defer r.Unlock()
	stat := r.peerStats[peerId]
	stat.QueueCount++
	stat.QueueSize += size
	r.peerStats[peerId] = stat
}

func (r *requestStat) AddSyncRequest(peerId string, size int) {
	r.Lock()
	defer r.Unlock()
	stat := r.peerStats[peerId]
	stat.SyncCount++
	stat.SyncSize += size
	r.peerStats[peerId] = stat
}

func (r *requestStat) RemoveSyncRequest(peerId string, size int) {
	r.Lock()
	defer r.Unlock()
	stat := r.peerStats[peerId]
	stat.SyncCount--
	stat.SyncSize -= size
	r.peerStats[peerId] = stat
}

func (r *requestStat) RemoveQueueRequest(peerId string, size int) {
	r.Lock()
	defer r.Unlock()
	stat := r.peerStats[peerId]
	stat.QueueCount--
	stat.QueueSize -= size
	r.peerStats[peerId] = stat
}

func (r *requestStat) CalcSize(msg *spacesyncproto.ObjectSyncMessage) int {
	return len(msg.Payload)
}

func (r *requestStat) QueueStat() spaceQueueStat {
	r.Lock()
	defer r.Unlock()
	var totalSize int
	var peerStats []peerStat
	for peerId, stat := range r.peerStats {
		totalSize += stat.QueueSize
		stat.PeerId = peerId
		peerStats = append(peerStats, stat)
	}
	return spaceQueueStat{
		TotalSize: totalSize,
		PeerStats: peerStats,
	}
}

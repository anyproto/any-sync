package sync

import (
	"sync"

	"golang.org/x/exp/slices"

	"github.com/anyproto/any-sync/app/debugstat"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type requestStat struct {
	sync.Mutex
	peerStats map[string]peerStat
	spaceId   string
}

func newRequestStat(spaceId string) *requestStat {
	return &requestStat{
		peerStats: make(map[string]peerStat),
		spaceId:   spaceId,
	}
}

type spaceQueueStat struct {
	SpaceId   string     `json:"space_id"`
	TotalSize int64      `json:"total_size"`
	PeerStats []peerStat `json:"peer_stats,omitempty"`
}

type summaryStat struct {
	TotalSize  int64            `json:"total_size"`
	QueueStats []spaceQueueStat `json:"sorted_stats,omitempty"`
}

type peerStat struct {
	QueueCount int    `json:"queue_count"`
	SyncCount  int    `json:"sync_count"`
	QueueSize  int64  `json:"queue_size"`
	SyncSize   int64  `json:"sync_size"`
	PeerId     string `json:"peer_id"`
}

func (r *requestStat) AddQueueRequest(peerId string, req *spacesyncproto.ObjectSyncMessage) {
	r.Lock()
	defer r.Unlock()
	stat := r.peerStats[peerId]
	stat.QueueCount++
	stat.QueueSize += int64(req.SizeVT())
	r.peerStats[peerId] = stat
}

func (r *requestStat) AddSyncRequest(peerId string, req *spacesyncproto.ObjectSyncMessage) {
	r.Lock()
	defer r.Unlock()
	stat := r.peerStats[peerId]
	stat.SyncCount++
	stat.SyncSize += int64(req.SizeVT())
	r.peerStats[peerId] = stat
}

func (r *requestStat) RemoveSyncRequest(peerId string, req *spacesyncproto.ObjectSyncMessage) {
	r.Lock()
	defer r.Unlock()
	stat := r.peerStats[peerId]
	stat.SyncCount--
	stat.SyncSize -= int64(req.SizeVT())
	r.peerStats[peerId] = stat
}

func (r *requestStat) RemoveQueueRequest(peerId string, req *spacesyncproto.ObjectSyncMessage) {
	r.Lock()
	defer r.Unlock()
	stat := r.peerStats[peerId]
	stat.QueueCount--
	stat.QueueSize -= int64(req.SizeVT())
	r.peerStats[peerId] = stat
}

func (r *requestStat) QueueStat() spaceQueueStat {
	r.Lock()
	defer r.Unlock()
	var totalSize int64
	var peerStats []peerStat
	for peerId, stat := range r.peerStats {
		totalSize += stat.QueueSize
		stat.PeerId = peerId
		peerStats = append(peerStats, stat)
	}
	slices.SortFunc(peerStats, func(first, second peerStat) int {
		firstTotalSize := first.QueueSize + first.SyncSize
		secondTotalSize := second.QueueSize + second.SyncSize
		if firstTotalSize > secondTotalSize {
			return -1
		} else if firstTotalSize == secondTotalSize {
			return 0
		} else {
			return 1
		}
	})
	return spaceQueueStat{
		SpaceId:   r.spaceId,
		TotalSize: totalSize,
		PeerStats: peerStats,
	}
}

func (r *requestStat) Aggregate(values []debugstat.StatValue) summaryStat {
	var totalSize int64
	var stats []spaceQueueStat
	for _, v := range values {
		stat, ok := v.Value.(spaceQueueStat)
		if !ok {
			continue
		}
		totalSize += stat.TotalSize
		stats = append(stats, stat)
	}
	slices.SortFunc(stats, func(first, second spaceQueueStat) int {
		if first.TotalSize > second.TotalSize {
			return -1
		} else if first.TotalSize == second.TotalSize {
			return 0
		} else {
			return 1
		}
	})
	return summaryStat{
		TotalSize:  totalSize,
		QueueStats: stats,
	}
}

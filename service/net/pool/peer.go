package pool

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
)

type peerEntry struct {
	peer     peer.Peer
	groupIds []string
	ready    chan struct{}
}

func (pe *peerEntry) addGroup(groupId string) (ok bool) {
	if slice.FindPos(pe.groupIds, groupId) != -1 {
		return false
	}
	pe.groupIds = append(pe.groupIds, groupId)
	return true
}

func (pe *peerEntry) removeGroup(groupId string) (ok bool) {
	if slice.FindPos(pe.groupIds, groupId) == -1 {
		return false
	}
	pe.groupIds = slice.Remove(pe.groupIds, groupId)
	return true
}

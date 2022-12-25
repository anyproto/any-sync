package objectsync

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/rpcerr"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	"time"
)

type StreamChecker interface {
	CheckResponsiblePeers()
	CheckPeerConnection(peerId string) (err error)
}

type streamChecker struct {
	spaceId       string
	connector     nodeconf.ConfConnector
	streamPool    StreamPool
	clientFactory spacesyncproto.ClientFactory
	log           *zap.Logger
	syncCtx       context.Context
	lastCheck     *atomic.Time
}

const streamCheckerInterval = time.Second * 10

func NewStreamChecker(
	spaceId string,
	connector nodeconf.ConfConnector,
	streamPool StreamPool,
	clientFactory spacesyncproto.ClientFactory,
	syncCtx context.Context,
	log *zap.Logger) StreamChecker {
	return &streamChecker{
		spaceId:       spaceId,
		connector:     connector,
		streamPool:    streamPool,
		clientFactory: clientFactory,
		log:           log,
		syncCtx:       syncCtx,
		lastCheck:     atomic.NewTime(time.Time{}),
	}
}

func (s *streamChecker) CheckResponsiblePeers() {
	lastCheck := s.lastCheck.Load()
	now := time.Now()
	if lastCheck.Add(streamCheckerInterval).After(now) {
		return
	}
	s.lastCheck.Store(now)

	var (
		activeNodeIds []string
		configuration = s.connector.Configuration()
	)
	nodeIds := configuration.NodeIds(s.spaceId)
	for _, nodeId := range nodeIds {
		if s.streamPool.HasActiveStream(nodeId) {
			s.log.Debug("has active stream for", zap.String("id", nodeId))
			activeNodeIds = append(activeNodeIds, nodeId)
			continue
		}
	}
	s.log.Debug("total streams", zap.Int("total", len(activeNodeIds)))
	newPeers, err := s.connector.DialInactiveResponsiblePeers(s.syncCtx, s.spaceId, activeNodeIds)
	if err != nil {
		s.log.Error("failed to dial peers", zap.Error(err))
		return
	}

	for _, p := range newPeers {
		err := s.createStream(p)
		if err != nil {
			log.With(zap.Error(err)).Error("failed to create stream")
			continue
		}
		s.log.Debug("reading stream for", zap.String("id", p.Id()))
	}
	return
}

func (s *streamChecker) CheckPeerConnection(peerId string) (err error) {
	if s.streamPool.HasActiveStream(peerId) {
		return
	}

	var (
		configuration = s.connector.Configuration()
		pool          = s.connector.Pool()
	)
	nodeIds := configuration.NodeIds(s.spaceId)
	// we don't know the address of the peer
	if !slices.Contains(nodeIds, peerId) {
		err = fmt.Errorf("don't know the address of peer %s", peerId)
		return
	}

	newPeer, err := pool.Dial(s.syncCtx, peerId)
	if err != nil {
		return
	}
	return s.createStream(newPeer)
}

func (s *streamChecker) createStream(p peer.Peer) (err error) {
	stream, err := s.clientFactory.Client(p).ObjectSyncStream(s.syncCtx)
	if err != nil {
		// so here probably the request is failed because there is no such space,
		// but diffService should handle such cases by sending pushSpace
		err = fmt.Errorf("failed to open stream: %w", rpcerr.Unwrap(err))
		return
	}

	// sending empty message for the server to understand from which space is it coming
	err = stream.Send(&spacesyncproto.ObjectSyncMessage{SpaceId: s.spaceId})
	if err != nil {
		err = fmt.Errorf("failed to send first message to stream: %w", rpcerr.Unwrap(err))
		return
	}
	err = s.streamPool.AddAndReadStreamAsync(stream)
	if err != nil {
		err = fmt.Errorf("failed to read from stream async: %w", err)
		return
	}
	return
}

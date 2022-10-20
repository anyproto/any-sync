package stream

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus"
	"github.com/cheggaaa/mb/v2"
	"go.uber.org/zap"
	"sync"
)

// Stream is a buffer that receives updates from object and gives back to a client
type Stream struct {
	id     uint64
	logIds map[string]struct{}
	mu     sync.Mutex
	mb     *mb.MB[consensus.Log]
	s      *service
}

// LogIds returns watched log ids
func (s *Stream) LogIds() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	logIds := make([][]byte, 0, len(s.logIds))
	for logId := range s.logIds {
		logIds = append(logIds, []byte(logId))
	}
	return logIds
}

// AddRecords adds new records to stream, called by objects
func (s *Stream) AddRecords(logId []byte, records []consensus.Record) (err error) {
	return s.mb.Add(consensus.Log{Id: logId, Records: records})
}

// WaitLogs wait for new log records
// empty returned slice means that stream is closed
func (s *Stream) WaitLogs() []consensus.Log {
	return s.mb.Wait()
}

// WatchIds adds given ids to subscription
func (s *Stream) WatchIds(ctx context.Context, logIds [][]byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, logId := range logIds {
		logIdKey := string(logId)
		if _, ok := s.logIds[logIdKey]; !ok {
			s.logIds[logIdKey] = struct{}{}
			if addErr := s.s.AddStream(ctx, logId, s); addErr != nil {
				log.Info("can't add stream for log", zap.Binary("logId", logId), zap.Error(addErr))
				_ = s.mb.Add(consensus.Log{
					Id:  logId,
					Err: addErr,
				})
			}
		}
	}
	return
}

// UnwatchIds removes given ids from subscription
func (s *Stream) UnwatchIds(ctx context.Context, logIds [][]byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, logId := range logIds {
		logIdKey := string(logId)
		if _, ok := s.logIds[logIdKey]; ok {
			delete(s.logIds, logIdKey)
			if remErr := s.s.RemoveStream(ctx, logId, s.id); remErr != nil {
				log.Warn("can't remove stream for log", zap.Binary("logId", logId), zap.Error(remErr))
			}
		}
	}
	return
}

// Close closes stream and unsubscribes all ids
func (s *Stream) Close() {
	_ = s.mb.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	for logId := range s.logIds {
		_ = s.s.RemoveStream(context.TODO(), []byte(logId), s.id)
	}
}

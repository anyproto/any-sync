package rpcstore

import (
	"context"
	"fmt"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"go.uber.org/zap"
	"sync"
)

var closedBlockChan chan blocks.Block

func init() {
	closedBlockChan = make(chan blocks.Block)
	close(closedBlockChan)
}

type store struct {
	s  *service
	cm *clientManager
	mu sync.RWMutex
}

func (s *store) Get(ctx context.Context, k cid.Cid) (b blocks.Block, err error) {
	t := newTask(ctx, taskOpGet, k, nil)
	if err = s.cm.Add(ctx, t); err != nil {
		return
	}
	select {
	case <-t.ready:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if err = t.Validate(); err != nil {
		return
	}
	return t.Block()
}

func (s *store) GetMany(ctx context.Context, ks []cid.Cid) <-chan blocks.Block {
	var readyCh = make(chan *task)
	var tasks = make([]*task, len(ks))
	for i, k := range ks {
		tasks[i] = newTask(ctx, taskOpGet, k, readyCh)
	}
	if err := s.cm.Add(ctx, tasks...); err != nil {
		log.Error("getMany: can't add tasks", zap.Error(err))
		return closedBlockChan
	}
	var result = make(chan blocks.Block)
	go func() {
		defer close(result)
		for i := 0; i < len(tasks); i++ {
			var t *task
			select {
			case <-ctx.Done():
				return
			case t = <-readyCh:
			}
			if err := t.Validate(); err != nil {
				// TODO: fallback
				log.Warn("received not valid block", zap.Error(err))
			}
			b, _ := t.Block()
			result <- b
		}
	}()
	return result
}

func (s *store) Add(ctx context.Context, bs []blocks.Block) error {
	var readyCh = make(chan *task)
	var tasks = make([]*task, len(bs))
	for i, b := range bs {
		tasks[i] = newTask(ctx, taskOpPut, b.Cid(), readyCh)
		tasks[i].data = b.RawData()
	}
	if err := s.cm.Add(ctx, tasks...); err != nil {
		return err
	}
	var errs = &ErrPartial{}
	for i := 0; i < len(tasks); i++ {
		select {
		case t := <-readyCh:
			if t.err != nil {
				errs.ErrorCids = append(errs.ErrorCids, ErrCid{Cid: t.cid, Err: t.err})
			} else {
				errs.SuccessCids = append(errs.SuccessCids, t.cid)
			}
		case <-ctx.Done():
			if len(errs.SuccessCids) > 0 {
				return errs
			}
			return ctx.Err()
		}
	}
	if len(errs.ErrorCids) > 0 {
		return errs
	}
	return nil
}

func (s *store) Delete(ctx context.Context, c cid.Cid) error {
	t := newTask(ctx, taskOpDelete, c, nil)
	if err := s.cm.Add(ctx, t); err != nil {
		return err
	}
	select {
	case t := <-t.ready:
		return t.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *store) Close() (err error) {
	return s.cm.Close()
}

type ErrPartial struct {
	SuccessCids []cid.Cid
	ErrorCids   []ErrCid
}

func (e ErrPartial) Error() string {
	return fmt.Sprintf("cid errors; success: %d; error: %d", len(e.SuccessCids), len(e.ErrorCids))
}

type ErrCid struct {
	Cid cid.Cid
	Err error
}

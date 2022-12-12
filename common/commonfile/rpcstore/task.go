package rpcstore

import (
	"context"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"time"
)

type taskOp uint

const (
	taskOpGet taskOp = iota
	taskOpPut
	taskOpDelete
)

func newTask(ctx context.Context, op taskOp, c cid.Cid, readyCh chan *task) *task {
	t := &task{
		cid:   c,
		ctx:   ctx,
		ready: readyCh,
		op:    op,
	}
	if t.ready == nil {
		t.ready = make(chan *task)
	}
	return t
}

type task struct {
	op          taskOp
	ctx         context.Context
	cid         cid.Cid
	data        []byte
	peerId      string
	spaceId     string
	denyPeerIds []string
	ready       chan *task
	startTime   time.Time
	err         error
}

func (t *task) Validate() error {
	if t.err != nil {
		return t.err
	}
	chkc, err := t.cid.Prefix().Sum(t.data)
	if err != nil {
		return err
	}
	if !chkc.Equals(t.cid) {
		return blocks.ErrWrongHash
	}
	return nil
}

func (t *task) Block() (blocks.Block, error) {
	return blocks.NewBlockWithCid(t.data, t.cid)
}

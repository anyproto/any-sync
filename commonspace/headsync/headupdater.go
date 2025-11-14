package headsync

import (
	"context"

	"github.com/cheggaaa/mb/v3"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
)

type headUpdater struct {
	updateFunc func(update headstorage.HeadsEntry)
	batcher    *mb.MB[headstorage.HeadsEntry]
}

func newHeadUpdater(update func(update headstorage.HeadsEntry)) *headUpdater {
	return &headUpdater{
		batcher:    mb.New[headstorage.HeadsEntry](0),
		updateFunc: update,
	}
}

func (hu *headUpdater) Add(update headstorage.HeadsEntry) error {
	return hu.batcher.Add(context.Background(), update)
}

func (hu *headUpdater) Run() {
	go hu.process()
}

func (hu *headUpdater) process() {
	for {
		msg, err := hu.batcher.WaitOne(context.Background())
		if err != nil {
			return
		}
		hu.updateFunc(msg)
	}
}

func (hu *headUpdater) Close() error {
	return hu.batcher.Close()
}

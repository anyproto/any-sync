package headsync

import (
	"context"

	"github.com/cheggaaa/mb/v3"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
)

type headUpdater struct {
	updateFunc func(update headstorage.HeadsUpdate)
	batcher    *mb.MB[headstorage.HeadsUpdate]
}

func newHeadUpdater(update func(update headstorage.HeadsUpdate)) *headUpdater {
	return &headUpdater{
		batcher: mb.New[headstorage.HeadsUpdate](0),
	}
}

func (hu *headUpdater) Add(update headstorage.HeadsUpdate) error {
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

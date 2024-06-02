package syncdeps

import "github.com/anyproto/any-sync/app"

const CName = "common.sync.syncdeps"

type SyncDepsFactory interface {
	app.Component
	SyncDeps() SyncDeps
}

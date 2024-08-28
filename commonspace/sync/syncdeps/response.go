package syncdeps

import (
	"github.com/anyproto/any-sync/util/multiqueue"
)

type Response interface {
	multiqueue.Sizeable
}

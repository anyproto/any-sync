package yamux

import "github.com/anyproto/any-sync/net/transport/yamux"

type configGetter interface {
	GetYamux() yamux.Config
}

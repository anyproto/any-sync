package debugserver

import debugServer2 "github.com/anyproto/any-sync/net/rpc/debugserver"

type configGetter interface {
	GetDebugServer() debugServer2.Config
}

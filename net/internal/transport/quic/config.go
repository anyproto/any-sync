package quic

import quic2 "github.com/anyproto/any-sync/net/transport/quic"

type configGetter interface {
	GetQuic() quic2.Config
}

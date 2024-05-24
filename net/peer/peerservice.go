package peer

type PeerService interface {
	SetPeerAddrs(peerId string, addrs []string)
	PreferQuic(prefer bool)
}

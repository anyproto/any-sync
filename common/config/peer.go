package config

type PeerList struct {
	MyId struct {
		PeerId  string `yaml:"peerId"`
		PrivKey string `yaml:"privKey"`
	} `yaml:"myId"`
	Remote []PeerRemote `yaml:"remote"`
}

type PeerRemote struct {
	PeerId string `yaml:"peerId"`
	Addr   string `yaml:"addr"`
}

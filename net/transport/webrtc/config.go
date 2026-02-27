package webrtc

type configGetter interface {
	GetWebRTC() Config
}

type Config struct {
	ListenAddrs     []string `yaml:"listenAddrs"`
	SignalPort      int      `yaml:"signalPort"`
	WriteTimeoutSec int      `yaml:"writeTimeoutSec"`
	DialTimeoutSec  int      `yaml:"dialTimeoutSec"`
	CloseTimeoutSec int      `yaml:"closeTimeoutSec"`
	// ICEServers is a list of STUN/TURN server URLs for ICE gathering.
	// Example: ["stun:stun.l.google.com:19302"]
	ICEServers []string `yaml:"iceServers"`
}

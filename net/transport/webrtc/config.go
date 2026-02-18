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
}

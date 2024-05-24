package quic

type Config struct {
	ListenAddrs        []string `yaml:"listenAddrs"`
	WriteTimeoutSec    int      `yaml:"writeTimeoutSec"`
	DialTimeoutSec     int      `yaml:"dialTimeoutSec"`
	MaxStreams         int64    `yaml:"maxStreams"`
	KeepAlivePeriodSec int      `yaml:"keepAlivePeriodSec"`
}

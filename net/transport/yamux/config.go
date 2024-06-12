package yamux

type Config struct {
	ListenAddrs        []string `yaml:"listenAddrs"`
	WriteTimeoutSec    int      `yaml:"writeTimeoutSec"`
	DialTimeoutSec     int      `yaml:"dialTimeoutSec"`
	KeepAlivePeriodSec int      `yaml:"keepAlivePeriodSec"`
}

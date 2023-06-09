package yamux

type configGetter interface {
	GetYamux() Config
}

type Config struct {
	ListenAddrs     []string `yaml:"listenAddrs"`
	WriteTimeoutSec int      `yaml:"writeTimeoutSec"`
	DialTimeoutSec  int      `yaml:"dialTimeoutSec"`
}

package webtransport

type configGetter interface {
	GetWebTransport() Config
}

type Config struct {
	ListenAddrs     []string `yaml:"listenAddrs"`
	Path            string   `yaml:"path"`
	CertFile        string   `yaml:"certFile"`
	KeyFile         string   `yaml:"keyFile"`
	WriteTimeoutSec int      `yaml:"writeTimeoutSec"`
	CloseTimeoutSec int      `yaml:"closeTimeoutSec"`
	DialTimeoutSec  int      `yaml:"dialTimeoutSec"`
	MaxStreams      int64    `yaml:"maxStreams"`
}

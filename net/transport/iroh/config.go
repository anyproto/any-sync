package iroh

type configGetter interface {
	GetIroh() Config
}

type Config struct {
	// RelayURLs are the home-relay candidates (https://host). Empty disables
	// relays: only direct paths are used and Ticket carries IP addresses.
	RelayURLs []string `yaml:"relayUrls"`
	// BindAddr is the UDP "ip:port" to bind. Empty binds an ephemeral port on
	// all interfaces.
	BindAddr           string `yaml:"bindAddr"`
	WriteTimeoutSec    int    `yaml:"writeTimeoutSec"`
	CloseTimeoutSec    int    `yaml:"closeTimeoutSec"`
	DialTimeoutSec     int    `yaml:"dialTimeoutSec"`
	MaxStreams         int64  `yaml:"maxStreams"`
	KeepAlivePeriodSec int    `yaml:"keepAlivePeriodSec"`
	MaxIdleTimeoutSec  int    `yaml:"maxIdleTimeoutSec"`
}

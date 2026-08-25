package iroh

type configGetter interface {
	GetIroh() Config
}

type Config struct {
	// RelayURLs are the home-relay candidates (https://host). Empty disables
	// relays: only direct paths are used and Ticket carries the bound
	// socket's IP addresses.
	RelayURLs []string `yaml:"relayUrls"`
	// InsecureRelay admits http:// relay URLs (plaintext WebSocket to the
	// relay). Development and tests only.
	InsecureRelay bool `yaml:"insecureRelay"`
	// BindAddr is the UDP "ip:port" to bind. Empty binds an ephemeral port on
	// all interfaces.
	BindAddr        string `yaml:"bindAddr"`
	WriteTimeoutSec int    `yaml:"writeTimeoutSec"`
	DialTimeoutSec  int    `yaml:"dialTimeoutSec"`
	MaxStreams      int64  `yaml:"maxStreams"`
	// InitialPacketSize overrides the initial QUIC packet size; 0 keeps the
	// go-iroh default.
	InitialPacketSize uint16 `yaml:"initialPacketSize"`
	// KeepAlivePeriodSec defaults to 25 and must stay below the idle timeout.
	KeepAlivePeriodSec int `yaml:"keepAlivePeriodSec"`
	// MaxIdleTimeoutSec is the QUIC idle timeout; 0 keeps the go-iroh
	// default (30s).
	MaxIdleTimeoutSec int `yaml:"maxIdleTimeoutSec"`
	// MaxInflightAccepts bounds inbound connections between arrival and the
	// end of the any-sync handshake; later arrivals are refused before any
	// handshake work. Defaults to 64.
	MaxInflightAccepts int `yaml:"maxInflightAccepts"`
}

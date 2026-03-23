package webtransport

type configGetter interface {
	GetWebTransport() Config
}

// Config holds the configuration for the WebTransport transport.
//
// Example YAML configuration:
//
//	webtransport:
//	  listenAddrs: ["0.0.0.0:443"]
//	  path: "/webtransport"
//	  certFile: "/path/to/cert.pem"
//	  keyFile: "/path/to/key.pem"
//	  writeTimeoutSec: 10
//	  closeTimeoutSec: 5
//	  dialTimeoutSec: 30
//	  maxStreams: 128
//
// Defaults (applied if not set): Path="/webtransport", CloseTimeoutSec=5,
// DialTimeoutSec=30, MaxStreams=128.
//
// CertFile and KeyFile must point to valid TLS certificate and key files
// (e.g. from Let's Encrypt / certbot). The server re-reads them on each new
// connection, so certificate rotation does not require a restart.
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

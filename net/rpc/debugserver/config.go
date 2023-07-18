package debugserver

type configGetter interface {
	GetDebugServer() Config
}

type Config struct {
	ListenAddr string `yaml:"listenAddr"`
}

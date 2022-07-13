package config

type GrpcServer struct {
	ListenAddrs []string `yaml:"listenAddrs"`
	TLS         bool     `yaml:"tls"`
	TLSCertFile string   `yaml:"tlsCertFile"`
	TLSKeyFile  string   `yaml:"tlsKeyFile"`
}

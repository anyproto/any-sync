package config

type GrpcServer struct {
	ListenAddrs []string `yaml:"listenAddrs"`
}

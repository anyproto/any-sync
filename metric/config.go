package metric

type configSource interface {
	GetMetric() Config
}

type Config struct {
	Addr string `yaml:"addr"`
}

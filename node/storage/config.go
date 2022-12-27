package storage

type configGetter interface {
	GetStorage() Config
}

type Config struct {
	Path string `yaml:"path"`
}

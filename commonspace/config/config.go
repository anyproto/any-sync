package config

type ConfigGetter interface {
	GetSpace() Config
}

type Config struct {
	GCTTL                int  `yaml:"gcTTL"`
	SyncPeriod           int  `yaml:"syncPeriod"`
	KeepTreeDataInMemory bool `yaml:"keepTreeDataInMemory"`
}

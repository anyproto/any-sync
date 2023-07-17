package config

type ConfigGetter interface {
	GetSpace() Config
}

type Config struct {
	GCTTL                int  `yaml:"gcTTL"`
	SyncPeriod           int  `yaml:"syncPeriod"`
	SyncLogPeriod        int  `yaml:"syncLogPeriod"`
	KeepTreeDataInMemory bool `yaml:"keepTreeDataInMemory"`
}

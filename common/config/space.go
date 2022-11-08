package config

type Space struct {
	GCTTL      int `yaml:"gcTTL"`
	SyncPeriod int `yaml:"syncPeriod"`
}

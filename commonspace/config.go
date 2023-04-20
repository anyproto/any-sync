package commonspace

type ConfigGetter interface {
	GetSpace() Config
}

type Config struct {
	GCTTL              int  `yaml:"gcTTL"`
	SyncPeriod         int  `yaml:"syncPeriod"`
	TreeNoInMemoryData bool `yaml:"treeNoInMemoryData"`
}

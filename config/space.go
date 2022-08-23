package config

type Space struct {
	GCTTL      int `json:"gcTTL"`
	SyncPeriod int `json:"syncPeriod"`
}

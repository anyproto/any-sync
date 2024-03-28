package limiter

type ConfigGetter interface {
	GetLimiterConf() Config
}

type Tokens struct {
	TokensPerSecond int `yaml:"rps"`
	MaxTokens       int `yaml:"burst"`
}

type Config struct {
	DefaultTokens  Tokens            `yaml:"default"`
	ResponseTokens map[string]Tokens `yaml:"rpc"`
}

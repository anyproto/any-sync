package config

type Mongo struct {
	Connect       string `yaml:"connect"`
	Database      string `yaml:"database"`
	LogCollection string `yaml:"logCollection"`
}

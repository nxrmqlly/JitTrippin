package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Project struct {
		Name string `toml:"name"`
	} `toml:"project"`

	Pipelines struct {
		Language string `toml:"language"`
		Dir      string `toml:"dir"`
	} `toml:"pipelines"`

	Engine struct {
		LogLevel string `toml:"log_level"`
	} `toml:"engine"`
}

func LoadConfig(path string) (*Config, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

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

// Parse takes a byte array of a TOML string and parses it into Config.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func LoadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

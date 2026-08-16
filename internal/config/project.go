package config

import (
	"bytes"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Project struct {
		Name string `toml:"name"`
	} `toml:"project"`

	Pipelines struct {
		Dir string `toml:"dir"`
	} `toml:"pipelines"`
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

// Save encodes cfg to TOML and writes it to path.
func Save(path string, cfg *Config) error {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o600)
}

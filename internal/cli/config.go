package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const defaultDaemon = "https://jt.ritam.cc"

type Config struct {
	Daemon string `toml:"daemon"`
}

func cfgPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "jittrippin", "user_config.toml"), nil
}

func loadCfg() (*Config, error) {
	p, err := cfgPath()
	if err != nil {
		return nil, err
	}
	cfg := &Config{Daemon: defaultDaemon}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := toml.Decode(string(b), cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func saveCfg(cfg *Config) error {
	p, err := cfgPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(p, buf.Bytes(), 0o600)
}

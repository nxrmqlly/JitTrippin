package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const DefaultDaemon = "https://jt.ritam.cc"

type UserConfig struct {
	Daemon string `toml:"daemon"`
}

func UserConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "jittrippin", "user_config.toml"), nil
}

func UserConfigExists() bool {
	p, err := UserConfigPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

func LoadUserConfig() (*UserConfig, error) {
	cfg := &UserConfig{Daemon: DefaultDaemon}
	p, err := UserConfigPath()
	if err != nil {
		return nil, err
	}
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

func SaveUserConfig(cfg *UserConfig) error {
	p, err := UserConfigPath()
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

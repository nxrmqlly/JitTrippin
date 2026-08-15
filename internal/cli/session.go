package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "jittrippin"
	keyringUser    = "session"
)

type Session struct {
	Token    string `json:"token"`
	Provider string `json:"provider"`
}

type sessionBackend interface {
	load() (*Session, error)
	save(*Session) error
	clear() error
}

type keyringBackend struct{}

func (b *keyringBackend) load() (*Session, error) {
	raw, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (b *keyringBackend) save(s *Session) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return keyring.Set(keyringService, keyringUser, string(raw))
}

func (b *keyringBackend) clear() error {
	return keyring.Delete(keyringService, keyringUser)
}

type fileBackend struct{}

func sessionPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "jittrippin", "session.json"), nil
}

func (b *fileBackend) load() (*Session, error) {
	p, err := sessionPath()
	if err != nil {
		return nil, err
	}
	byt, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(byt, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (b *fileBackend) save(s *Session) error {
	p, err := sessionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(p, raw, 0o600)
}

func (b *fileBackend) clear() error {
	p, err := sessionPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

type hybridBackend struct {
	primary  sessionBackend
	fallback sessionBackend
}

func (h hybridBackend) load() (*Session, error) {
	if s, err := h.primary.load(); err == nil {
		return s, nil
	}
	s, err := h.fallback.load()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (h hybridBackend) save(s *Session) error {
	if err := h.primary.save(s); err == nil {
		_ = h.fallback.clear()
		return nil
	}
	return h.fallback.save(s)
}

func (h hybridBackend) clear() error {
	_ = h.primary.clear()
	return h.fallback.clear()
}

var sessionStore = hybridBackend{primary: &keyringBackend{}, fallback: &fileBackend{}}

func loadSession() (*Session, error) { return sessionStore.load() }
func saveSession(s *Session) error   { return sessionStore.save(s) }
func clearSession() error            { return sessionStore.clear() }

func hasSession() bool {
	s, _ := loadSession()
	return s != nil && s.Token != ""
}

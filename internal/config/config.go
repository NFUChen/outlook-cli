// Package config manages ~/.outlook-cli/config.toml.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds the persisted CLI configuration.
type Config struct {
	ClientID string `toml:"client_id,omitempty"`
	TenantID string `toml:"tenant_id,omitempty"`
}

// Dir returns the config directory (~/.outlook-cli), creating it if needed.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".outlook-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// FilePath returns the config file path without creating any directories.
func FilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".outlook-cli", "config.toml"), nil
}

// Load reads the config from disk, returning a zero Config if not found.
func Load() (Config, error) {
	path, err := FilePath()
	if err != nil {
		return Config{}, fmt.Errorf("Cannot read config: %v", err)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("Cannot read config: %v", err)
	}
	var c Config
	if err := toml.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("Corrupt config file: %s", path)
	}
	return c, nil
}

// Save writes the config to disk with 0600 permissions.
func Save(c Config) error {
	dir, err := Dir()
	if err != nil {
		return fmt.Errorf("Cannot save config: %v", err)
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(c); err != nil {
		return fmt.Errorf("Cannot save config: %v", err)
	}
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("Cannot save config: %v", err)
	}
	return nil
}

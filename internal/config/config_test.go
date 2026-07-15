package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingReturnsZero(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.ClientID != "" || c.TenantID != "" {
		t.Errorf("expected zero config, got %+v", c)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	want := Config{ClientID: "abc-123", TenantID: "common"}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSavePermissions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := Save(Config{ClientID: "x"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	dirInfo, err := os.Stat(filepath.Join(home, ".outlook-cli"))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir perm = %o, want 700", perm)
	}

	fileInfo, err := os.Stat(filepath.Join(home, ".outlook-cli", "config.toml"))
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perm = %o, want 600", perm)
	}
}

func TestLoadCorruptFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".outlook-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("not [valid toml"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for corrupt config")
	}
	if !strings.Contains(err.Error(), "Corrupt config file:") {
		t.Errorf("unexpected error: %v", err)
	}
}

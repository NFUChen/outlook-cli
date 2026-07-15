package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestSaveLoadTokenRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	want := &oauth2.Token{
		AccessToken:  "access",
		RefreshToken: "refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour).Round(time.Second),
	}
	if err := SaveToken(want); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
	got, err := LoadToken()
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSaveTokenPermissions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := SaveToken(&oauth2.Token{AccessToken: "a"}); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
	info, err := os.Stat(filepath.Join(home, ".outlook-cli", TokenFilename))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("token perm = %o, want 600", perm)
	}
}

func TestIsAuthenticated(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if IsAuthenticated() {
		t.Error("expected false with no token")
	}

	// Expired token without refresh token: not authenticated.
	expired := &oauth2.Token{AccessToken: "a", Expiry: time.Now().Add(-time.Hour)}
	if err := SaveToken(expired); err != nil {
		t.Fatal(err)
	}
	if IsAuthenticated() {
		t.Error("expected false for expired token without refresh token")
	}

	// Expired token with refresh token: authenticated (refreshable).
	expired.RefreshToken = "r"
	if err := SaveToken(expired); err != nil {
		t.Fatal(err)
	}
	if !IsAuthenticated() {
		t.Error("expected true for token with refresh token")
	}

	// Valid token.
	valid := &oauth2.Token{AccessToken: "a", Expiry: time.Now().Add(time.Hour)}
	if err := SaveToken(valid); err != nil {
		t.Fatal(err)
	}
	if !IsAuthenticated() {
		t.Error("expected true for valid token")
	}
}

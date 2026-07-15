package auth

import (
	"encoding/json"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"

	"github.com/mhattingpete/outlook-cli/internal/config"
)

// TokenFilename is the token file name inside the config directory.
const TokenFilename = "token.json"

// TokenPath returns the token file path, creating the config directory if needed.
func TokenPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, TokenFilename), nil
}

// LoadToken reads the stored token from disk.
func LoadToken() (*oauth2.Token, error) {
	path, err := TokenPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tok oauth2.Token
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

// SaveToken writes the token to disk with 0600 permissions.
func SaveToken(tok *oauth2.Token) error {
	path, err := TokenPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// IsAuthenticated reports whether a usable token exists on disk.
func IsAuthenticated() bool {
	tok, err := LoadToken()
	if err != nil {
		return false
	}
	return tok.Valid() || tok.RefreshToken != ""
}

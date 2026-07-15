package auth

import (
	"context"
	"errors"

	"golang.org/x/oauth2"
)

// ErrNotAuthenticated is returned when no token is stored or refresh fails.
var ErrNotAuthenticated = errors.New("Not authenticated. Run: outlook auth login")

// TokenSource returns a token source that refreshes expired tokens and
// persists refreshed tokens back to disk.
func TokenSource(ctx context.Context, clientID, tenantID string) (oauth2.TokenSource, error) {
	tok, err := LoadToken()
	if err != nil {
		return nil, ErrNotAuthenticated
	}
	cfg := oauthConfig(clientID, tenantID)
	return &fileTokenSource{
		src:        cfg.TokenSource(ctx, tok),
		lastAccess: tok.AccessToken,
	}, nil
}

type fileTokenSource struct {
	src        oauth2.TokenSource
	lastAccess string
}

func (f *fileTokenSource) Token() (*oauth2.Token, error) {
	tok, err := f.src.Token()
	if err != nil {
		return nil, err
	}
	if tok.AccessToken != f.lastAccess {
		if err := SaveToken(tok); err != nil {
			return nil, err
		}
		f.lastAccess = tok.AccessToken
	}
	return tok, nil
}

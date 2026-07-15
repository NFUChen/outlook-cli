package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestTokenSourceNotAuthenticated(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_, err := TokenSource(context.Background(), "client", "common")
	if err != ErrNotAuthenticated {
		t.Errorf("got %v, want ErrNotAuthenticated", err)
	}
}

func TestTokenSourceRefreshPersistsToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var gotRefresh string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		gotRefresh = r.Form.Get("refresh_token")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":3600}`)
	}))
	defer srv.Close()

	oldBase := AuthorityBase
	AuthorityBase = srv.URL
	defer func() { AuthorityBase = oldBase }()

	expired := &oauth2.Token{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		Expiry:       time.Now().Add(-time.Hour),
	}
	if err := SaveToken(expired); err != nil {
		t.Fatal(err)
	}

	src, err := TokenSource(context.Background(), "client", "common")
	if err != nil {
		t.Fatalf("TokenSource: %v", err)
	}

	tok, err := src.Token()
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok.AccessToken != "new-access" {
		t.Errorf("access token = %q, want new-access", tok.AccessToken)
	}
	if gotRefresh != "old-refresh" {
		t.Errorf("server received refresh_token %q, want old-refresh", gotRefresh)
	}

	// The refreshed token must be written back to disk.
	saved, err := LoadToken()
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}
	if saved.AccessToken != "new-access" {
		t.Errorf("saved access token = %q, want new-access", saved.AccessToken)
	}
	if saved.RefreshToken != "new-refresh" {
		t.Errorf("saved refresh token = %q, want new-refresh", saved.RefreshToken)
	}
}

func TestTokenSourceValidTokenNoRefresh(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("token endpoint should not be called for a valid token")
	}))
	defer srv.Close()

	oldBase := AuthorityBase
	AuthorityBase = srv.URL
	defer func() { AuthorityBase = oldBase }()

	valid := &oauth2.Token{AccessToken: "still-good", Expiry: time.Now().Add(time.Hour)}
	if err := SaveToken(valid); err != nil {
		t.Fatal(err)
	}

	src, err := TokenSource(context.Background(), "client", "common")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := src.Token()
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "still-good" {
		t.Errorf("access token = %q, want still-good", tok.AccessToken)
	}
}

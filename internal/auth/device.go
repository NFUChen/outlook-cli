// Package auth implements the OAuth2 device code flow and token storage.
package auth

import (
	"context"
	"fmt"
	"io"

	"golang.org/x/oauth2"
)

// AuthorityBase is the Microsoft identity platform base URL (overridable in tests).
var AuthorityBase = "https://login.microsoftonline.com"

// Scopes requested for Microsoft Graph access.
var Scopes = []string{
	"https://graph.microsoft.com/Mail.ReadWrite",
	"https://graph.microsoft.com/Mail.Send",
	"https://graph.microsoft.com/Calendars.ReadWrite",
	"offline_access",
}

func oauthConfig(clientID, tenantID string) *oauth2.Config {
	return &oauth2.Config{
		ClientID: clientID,
		Scopes:   Scopes,
		Endpoint: oauth2.Endpoint{
			DeviceAuthURL: fmt.Sprintf("%s/%s/oauth2/v2.0/devicecode", AuthorityBase, tenantID),
			TokenURL:      fmt.Sprintf("%s/%s/oauth2/v2.0/token", AuthorityBase, tenantID),
		},
	}
}

// Login runs the device code flow, printing sign-in instructions to out,
// and saves the acquired token to disk.
func Login(ctx context.Context, clientID, tenantID string, out io.Writer) error {
	cfg := oauthConfig(clientID, tenantID)

	da, err := cfg.DeviceAuth(ctx)
	if err != nil {
		return fmt.Errorf("Device code flow failed: %v", err)
	}

	fmt.Fprintf(out, "\nTo sign in, visit: %s\n", da.VerificationURI)
	fmt.Fprintf(out, "Enter code: %s\n\n", da.UserCode)

	tok, err := cfg.DeviceAccessToken(ctx, da)
	if err != nil {
		return err
	}

	if err := SaveToken(tok); err != nil {
		return fmt.Errorf("Token acquired but failed to save: %v", err)
	}
	return nil
}

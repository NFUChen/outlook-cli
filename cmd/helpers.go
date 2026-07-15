package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mhattingpete/outlook-cli/internal/auth"
	"github.com/mhattingpete/outlook-cli/internal/config"
	"github.com/mhattingpete/outlook-cli/internal/graph"
)

// defaultNewClient builds a Graph client from stored config and token.
func defaultNewClient() (*graph.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.ClientID == "" {
		return nil, errors.New("Not configured. Run: outlook auth login --client-id <ID>")
	}
	tenant := cfg.TenantID
	if tenant == "" {
		tenant = "common"
	}
	tokens, err := auth.TokenSource(context.Background(), cfg.ClientID, tenant)
	if err != nil {
		return nil, err
	}
	return graph.NewClient(tokens), nil
}

// parseDate parses a YYYY-MM-DD date string in the local timezone.
func parseDate(value string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("Invalid date format: %s (expected YYYY-MM-DD)", value)
	}
	return t, nil
}

// parseDateTime parses a "YYYY-MM-DD HH:MM" datetime string in the local timezone.
func parseDateTime(value string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02 15:04", value, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("Invalid datetime format: %s (expected YYYY-MM-DD HH:MM)", value)
	}
	return t, nil
}

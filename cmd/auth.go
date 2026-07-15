package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mhattingpete/outlook-cli/internal/auth"
	"github.com/mhattingpete/outlook-cli/internal/config"
)

func newAuthCmd(app *App) *cobra.Command {
	c := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
	}
	c.AddCommand(newAuthLoginCmd(app), newAuthLogoutCmd(app), newAuthStatusCmd(app))
	return c
}

func newAuthLoginCmd(app *App) *cobra.Command {
	var clientID, tenantID string
	c := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Microsoft via device code flow.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if clientID != "" {
				cfg.ClientID = clientID
				cfg.TenantID = tenantID
				if err := config.Save(cfg); err != nil {
					return err
				}
			} else {
				clientID = cfg.ClientID
			}

			if clientID == "" {
				return errors.New("No client ID found. Run: outlook auth login --client-id <ID>")
			}

			if cfg.TenantID != "" {
				tenantID = cfg.TenantID
			}

			if err := app.Authenticate(cmd.Context(), clientID, tenantID, app.Printer.Out); err != nil {
				app.Printer.Error(err.Error())
				return errors.New("Authentication failed.")
			}
			app.Printer.Success("Authenticated successfully.")
			return nil
		},
	}
	c.Flags().StringVar(&clientID, "client-id", "", "Azure app client ID")
	c.Flags().StringVar(&tenantID, "tenant-id", "common", "Azure tenant ID")
	return c
}

func newAuthLogoutCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials.",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := auth.TokenPath()
			if err != nil {
				return fmt.Errorf("Failed to remove token: %v", err)
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				app.Printer.Println("No token file found; already logged out.")
				return nil
			}
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("Failed to remove token: %v", err)
			}
			app.Printer.Success("Logged out — token removed.")
			return nil
		},
	}
}

func newAuthStatusCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current authentication status and config.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			clientID := cfg.ClientID
			if clientID == "" {
				clientID = "not set"
			}
			tenantID := cfg.TenantID
			if tenantID == "" {
				tenantID = "not set"
			}
			app.Printer.Field("Client ID", clientID)
			app.Printer.Field("Tenant ID", tenantID)

			if app.IsAuthenticated() {
				app.Printer.Success("Authenticated.")
			} else {
				app.Printer.Error("Not authenticated.")
			}
			return nil
		},
	}
}

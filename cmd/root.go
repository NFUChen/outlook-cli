// Package cmd assembles the outlook CLI commands.
package cmd

import (
	"context"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/mhattingpete/outlook-cli/internal/auth"
	"github.com/mhattingpete/outlook-cli/internal/display"
	"github.com/mhattingpete/outlook-cli/internal/graph"
)

// App holds the injectable dependencies for all commands.
type App struct {
	Printer         *display.Printer
	NewClient       func() (*graph.Client, error)
	Authenticate    func(ctx context.Context, clientID, tenantID string, out io.Writer) error
	IsAuthenticated func() bool
	Now             func() time.Time
}

// NewApp returns an App wired to the real config, auth, and Graph API.
func NewApp() *App {
	return &App{
		Printer:         display.NewPrinter(),
		NewClient:       defaultNewClient,
		Authenticate:    auth.Login,
		IsAuthenticated: auth.IsAuthenticated,
		Now:             time.Now,
	}
}

// NewRoot builds the root command tree.
func NewRoot(app *App, version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "outlook",
		Short:         "CLI for Microsoft Outlook",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("outlook-cli {{.Version}}\n")
	root.AddCommand(newAuthCmd(app), newMailCmd(app), newCalCmd(app))
	return root
}

// Execute runs the CLI, printing any error as "Error: ...".
func Execute(version string) error {
	app := NewApp()
	root := NewRoot(app, version)
	if err := root.Execute(); err != nil {
		app.Printer.Error(err.Error())
		return err
	}
	return nil
}

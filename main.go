// Command outlook is a CLI for Microsoft Outlook (mail and calendar).
package main

import (
	"os"

	"github.com/mhattingpete/outlook-cli/cmd"
)

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := cmd.Execute(version); err != nil {
		os.Exit(1)
	}
}

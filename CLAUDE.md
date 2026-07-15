# CLAUDE.md — outlook-cli

## Project Overview

CLI tool for Microsoft Outlook (Work/School accounts), written in Go. Talks directly to the Microsoft Graph REST API (`net/http`), uses Cobra for the CLI framework, and ships as a single static binary.

## Quick Reference

```bash
go test ./... -race               # Run tests
go vet ./...                      # Vet
go run . --help                   # CLI help
go build -trimpath -ldflags "-s -w" -o outlook .   # Release build
```

## Project Structure

```
main.go                      # Entry point; version injected via -ldflags "-X main.version=..."
cmd/
├── root.go                  # App struct (injectable deps), root command, Execute()
├── helpers.go               # defaultNewClient (config+token → graph.Client), date parsing
├── auth.go                  # outlook auth login|logout|status
├── mail.go                  # outlook mail search|read|send|reply|mark
└── cal.go                   # outlook cal list|read|create
internal/
├── config/config.go         # Config{ClientID, TenantID}, ~/.outlook-cli/config.toml (dir 0700, file 0600)
├── auth/
│   ├── device.go            # OAuth2 device code flow (golang.org/x/oauth2)
│   ├── token.go             # token.json load/save (0600), IsAuthenticated
│   └── source.go            # fileTokenSource: auto-refresh + persist refreshed tokens
├── graph/
│   ├── client.go            # Client{HTTP, BaseURL, Tokens}, do() JSON helper, APIError, ErrNotFound
│   ├── query.go             # MailQuery/EventQuery → url.Values ($search/$filter/$top/$orderby/$select)
│   ├── mail.go              # ResolveFolder, ListMessages, GetMessage, SendMail, Reply, SetRead
│   └── calendar.go          # ListEvents, GetEvent, CreateEvent
└── display/
    ├── display.go           # Printer{Out, Err, Color}: Error/Success/Warn, table, panel
    ├── html.go              # StripHTML / LooksLikeHTML
    ├── mail.go              # MailTable, MailDetail
    └── calendar.go          # EventTable, EventDetail
```

## Architecture

- **Auth flow**: OAuth2 device code via `golang.org/x/oauth2` (`Config.DeviceAuth` + `DeviceAccessToken`) against `https://login.microsoftonline.com/{tenant}/oauth2/v2.0/`. Token stored in `~/.outlook-cli/token.json`; `fileTokenSource` auto-refreshes and writes refreshed tokens back to disk.
- **Dependency injection**: `cmd.App` holds `Printer`, `NewClient`, `Authenticate`, `IsAuthenticated`, `Now`. Tests inject a `graph.Client{BaseURL: testServer.URL}` and fake clocks.
- **Errors**: Commands return errors from `RunE`; `cmd.Execute` prints them as `Error: ...` (stderr) and main exits 1. `graph.ErrNotFound` (404) is mapped to user-facing messages like `Message not found: <id>` in cmd.
- **Display**: stdlib only — `text/tabwriter` tables, hand-rolled ANSI colors (auto-detect tty + `NO_COLOR`), panel-style detail views.

## Key Patterns

- **Graph queries**: `MailQuery`/`EventQuery` build `url.Values`. `$search` and `$filter`/`$orderby` are mutually exclusive (Graph API limitation) — when a text query is given with filter flags, a yellow Warning is printed and filters are ignored.
- **OData escaping**: single quotes doubled in string literals (`escapeODataString`); `$search` values wrapped in double quotes.
- **Folder resolution**: `"Inbox"` maps to the well-known `inbox` id; other names are looked up via `GET /me/mailFolders?$filter=displayName eq '...'`.
- **Times**: dates parsed in local timezone, sent to Graph as UTC. Event creation sends `{dateTime, timeZone: "UTC"}`. Display converts UTC back to local (except all-day events, which keep wall-clock time).
- **Display safety**: Graph objects use pointer fields (`*Recipient`, `*ItemBody`, ...) — display helpers nil-check everything.

## Testing Strategy

All tests use stdlib `testing` (no assertion libraries):

1. **Unit tests**: `internal/config` and `internal/auth` use `t.Setenv("HOME", t.TempDir())`; auth refresh tests use `httptest` fake token endpoints via the `AuthorityBase` package var.
2. **Graph tests**: `httptest.Server` verifies method/path/query/headers and response decoding; table-driven query builder tests.
3. **CLI integration tests** (`cmd/cmd_test.go`): build the command tree via `NewRoot(app, ...)` with an injected test server client, run with `SetArgs`, assert on captured Printer output and returned errors.

## Dependencies

- `github.com/spf13/cobra` — CLI framework (sub-command groups)
- `golang.org/x/oauth2` — device code flow + token refresh
- `github.com/BurntSushi/toml` — config.toml read/write

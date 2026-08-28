# outlook-cli

A command-line tool for Microsoft Outlook (Work/School accounts). Search, read, send, draft, and reply to emails. List, read, and create calendar events. Written in Go — ships as a single small binary that talks directly to the [Microsoft Graph API](https://learn.microsoft.com/en-us/graph/overview).

## Prerequisites

- An Azure App Registration (free) with **Microsoft Graph** delegated permissions:
  - `Mail.ReadWrite`
  - `Mail.Send`
  - `Calendars.ReadWrite`

### Azure App Setup

1. Go to [Azure Portal > App registrations](https://portal.azure.com/#blade/Microsoft_AAD_RegisteredApps/ApplicationsListBlade)
2. Click **New registration**
3. Name it anything (e.g. "Outlook CLI"), set **Supported account types** to your preference
4. Under **Authentication > Advanced settings**, set **Allow public client flows** to **Yes** and save
5. Under **API permissions**, add the three **Delegated** permissions listed above
6. If your tenant requires admin consent, have an admin grant consent for the app
7. Copy the **Application (client) ID** — you'll need it for `outlook auth login`

## Installation

Download a prebuilt binary from the [releases page](https://github.com/mhattingpete/outlook-cli/releases), or build from source:

```bash
git clone https://github.com/mhattingpete/outlook-cli.git
cd outlook-cli
go build -trimpath -ldflags "-s -w" -o outlook .
```

## Getting Started

```bash
# Authenticate with your Azure app
outlook auth login --client-id YOUR_CLIENT_ID

# Follow the device code flow — open the URL and enter the code

# Check status
outlook auth status
```

## Commands

### Authentication

```bash
outlook auth login [--client-id ID] [--tenant-id ID]   # Authenticate via device code flow
outlook auth logout                                      # Remove stored token
outlook auth status                                      # Show auth status and config
```

### Email

```bash
# Search / list messages
outlook mail search                                      # List recent inbox messages
outlook mail search "quarterly report"                   # Full-text search
outlook mail search --unread                             # Unread messages only
outlook mail search --from alice@company.com             # Filter by sender (--sender also works)
outlook mail search --start-date 2025-01-01 --end-date 2025-02-01
outlook mail search --important --has-attachments        # Combine filters
outlook mail search --folder "Sent Items" --limit 10     # Different folder

# Read a message
outlook mail read MESSAGE_ID

# Send a message
outlook mail send --to bob@company.com --subject "Hello" --body "Hi Bob!"
outlook mail send --to bob@company.com --cc carol@company.com --subject "Update" --body "FYI"

# Send HTML email
outlook mail send --to bob@company.com --subject "HTML Test" --body "<h1>Hello</h1><p>This is HTML</p>" --html

# Save a draft (does not send)
outlook mail draft --to bob@company.com --subject "WIP" --body "Draft body"
outlook mail draft --to a@x.com --to b@x.com --cc c@x.com --bcc d@x.com --subject "Multi" --body "..." --importance high

# Save HTML draft
outlook mail draft --to bob@company.com --subject "HTML Draft" --body "<p>Draft with <b>HTML</b></p>" --html

# Reply to a message
outlook mail reply MESSAGE_ID --body "Thanks for the update!"
outlook mail reply MESSAGE_ID --body "Noted, thanks." --reply-all

# Reply with HTML
outlook mail reply MESSAGE_ID --body "<p>Thanks for the <em>detailed</em> update!</p>" --html

# Reply with attachment
outlook mail reply MESSAGE_ID --body "See attached" --attach response.pdf

# Create reply draft (saves to Drafts without sending)
outlook mail reply MESSAGE_ID --body "Let me review this first" --draft
outlook mail reply MESSAGE_ID --body "<b>Draft reply</b>" --draft --html --reply-all

# Forward a message
outlook mail forward MESSAGE_ID --to bob@company.com
outlook mail forward MESSAGE_ID --to bob@company.com --cc carol@company.com --body "See below"

# Forward with attachment
outlook mail forward MESSAGE_ID --to bob@company.com --attach response.pdf

# Create forward draft (saves to Drafts without sending)
outlook mail forward MESSAGE_ID --to bob@company.com --body "FYI" --draft

# Mark as read/unread
outlook mail mark MESSAGE_ID                             # Mark as read (default)
outlook mail mark MESSAGE_ID --read                      # Same, explicit
outlook mail mark MESSAGE_ID --unread                    # Mark as unread

# Attachments - send with attachments
outlook mail send --to bob@company.com --subject "Report" --body "See attached" --attach report.pdf
outlook mail send --to bob@company.com --subject "Files" --body "Multiple files" --attach doc1.pdf --attach doc2.xlsx

# Attachments - draft with attachments
outlook mail draft --to bob@company.com --subject "WIP" --body "Draft with files" --attach notes.txt

# Attachments - list attachments on a message
outlook mail attachment list MESSAGE_ID

# Attachments - download an attachment
outlook mail attachment download MESSAGE_ID ATTACHMENT_ID
outlook mail attachment download MESSAGE_ID ATTACHMENT_ID --output ~/Downloads/renamed.pdf
```

> **Note on HTML emails**: When using `--html`, you're responsible for providing valid HTML and escaping content as needed. The CLI passes your HTML directly to the Microsoft Graph API.

### Calendar

```bash
# List events (default: next 7 days)
outlook cal list
outlook cal list --start 2025-03-01 --end 2025-03-31    # Custom date range
outlook cal list --subject "standup"                     # Filter by subject
outlook cal list --location "Room A"                     # Filter by location
outlook cal list --all-day                               # All-day events only
outlook cal list --recurring                             # Recurring events only
outlook cal list --organizer boss@company.com            # Filter by organizer

# Read event details (shows attendees, recurrence, etc.)
outlook cal read EVENT_ID

# Create an event
outlook cal create --subject "Lunch" --start "2025-02-08 12:00" --end "2025-02-08 13:00"
outlook cal create --subject "Workshop" \
  --start "2025-02-10 09:00" --end "2025-02-10 17:00" \
  --body "Full day workshop" --location "Conference Room B"
```

## Using with Claude Code

outlook-cli works with [Claude Code](https://docs.anthropic.com/en/docs/claude-code) out of the box.

### Claude Code Skill

This repo ships a skill at [`.claude/skills/outlook-cli/SKILL.md`](.claude/skills/outlook-cli/SKILL.md) that teaches Claude the full command surface — flags, message/event ID handling, and the search-vs-filter rules.

- **Inside this repo**: the skill is picked up automatically.
- **Everywhere else**: copy it to your global skills directory:

```bash
mkdir -p ~/.claude/skills
cp -r .claude/skills/outlook-cli ~/.claude/skills/
```

### Example prompts

Here are example prompts and what Claude does with them:

```bash
# ── Email triage ──────────────────────────────────────────
# Prompt: "Show me unread emails from alice"
outlook mail search --unread --from alice@company.com

# Prompt: "Read the first message"
outlook mail read aB3x-def4-5678-gh90

# Prompt: "Reply saying I'll handle it today"
outlook mail reply aB3x-def4-5678-gh90 --body "I'll handle this today."

# Prompt: "Mark it as read"
outlook mail mark aB3x-def4-5678-gh90

# ── Calendar management ──────────────────────────────────
# Prompt: "What's on my calendar this week?"
outlook cal list

# Prompt: "Any recurring meetings?"
outlook cal list --recurring

# Prompt: "Schedule a team lunch Friday at noon"
outlook cal create --subject "Team Lunch" \
  --start "2025-02-14 12:00" --end "2025-02-14 13:00" \
  --location "Cafeteria"

# ── Search and respond ───────────────────────────────────
# Prompt: "Find emails about the project proposal with attachments"
outlook mail search "project proposal" --has-attachments

# Prompt: "Reply-all to the latest with my feedback"
outlook mail reply msg-id --reply-all --body "Looks good — approved."
```

## Configuration

Config and tokens are stored in `~/.outlook-cli/`:

```text
~/.outlook-cli/
├── config.toml          # client_id, tenant_id
└── token.json           # OAuth token (auto-managed, auto-refreshed)
```

> Upgrading from the Python version? The config file is compatible, but the token format changed — run `outlook auth login` once to re-authenticate.

## Development

```bash
# Run tests
go test ./... -race

# Vet
go vet ./...

# Run the CLI
go run . --help
```

## License

MIT

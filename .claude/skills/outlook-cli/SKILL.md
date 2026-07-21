---
name: outlook-cli
description: Operate Microsoft Outlook (Work/School) from the terminal via the `outlook` CLI — search/read/send/draft/reply to email, mark read/unread, and list/read/create calendar events. Use this skill whenever the user asks about their Outlook inbox, unread mail, sending or replying to email, meetings, or their calendar.
---

# outlook-cli

Drive Microsoft Outlook through the `outlook` binary (Microsoft Graph API under the hood).

## Preflight

Before the first Outlook operation in a session, verify auth:

```bash
outlook auth status
```

If it reports "Not authenticated", ask the user to run `outlook auth login` (device code flow — it requires interactive browser login, so don't run it yourself unless the user asks).

## Command Reference

### Email

```bash
outlook mail search [QUERY] [flags]      # List/search messages (default folder: Inbox, limit 25)
  --folder "Sent Items"                  # Any folder by display name
  --limit N
  --from EMAIL                           # Filter by sender (--sender is an alias)
  --start-date YYYY-MM-DD                # Received after
  --end-date YYYY-MM-DD                  # Received before
  --unread --important --has-attachments # Boolean filters, combinable

outlook mail read MESSAGE_ID             # Full message with body (HTML stripped)
outlook mail send --to EMAIL --subject S --body B [--cc EMAIL] [--html] [--attach FILE]...
outlook mail draft [--to EMAIL]... [--cc EMAIL]... [--bcc EMAIL]... [--subject S] [--body B] \
  [--importance low|normal|high] [--html] [--attach FILE]...
  # Saves to Drafts, does NOT send; needs at least one field
outlook mail reply MESSAGE_ID --body B [--reply-all] [--html] [--draft]
  # --draft: saves reply to Drafts instead of sending immediately
outlook mail mark MESSAGE_ID [--read|--unread]   # Default: mark as read

# Attachment commands
outlook mail attachment list MESSAGE_ID              # List all attachments with IDs
outlook mail attachment download MESSAGE_ID ATT_ID  # Download attachment
  [-o FILE]                                          # --output: custom filename
```

### Calendar

```bash
outlook cal list [flags]                 # Default range: today → +7 days, limit 25
  --start YYYY-MM-DD --end YYYY-MM-DD
  --subject KEYWORD --location KEYWORD --organizer EMAIL
  --all-day --recurring

outlook cal read EVENT_ID                # Attendees, recurrence, description
outlook cal create --subject S --start "YYYY-MM-DD HH:MM" --end "YYYY-MM-DD HH:MM" \
  [--body TEXT] [--location TEXT]
```

## Critical Rules

1. **Text search and filters are mutually exclusive.** If a `QUERY` argument is given, all filter flags (`--from`, `--unread`, `--start-date`, ...) are ignored (Graph API limitation; the CLI prints a warning). Pick one:
   - Content keywords → `outlook mail search "keyword"`
   - Structured criteria → flags only, no positional query
2. **IDs come from tables.** `mail search` and `cal list` print an `ID` column (long opaque Graph IDs). Pass them verbatim to `read` / `reply` / `mark`. Never invent or truncate an ID; if you don't have one, search/list first.
3. **Times are local.** Dates are `YYYY-MM-DD`; `cal create` datetimes are `"YYYY-MM-DD HH:MM"` in the user's local timezone (quote them — they contain a space). Output is also shown in local time.
4. **HTML emails.** Use `--html` flag with `send`, `draft`, or `reply` to send HTML-formatted emails. Without this flag, body is sent as plain text. When using `--html`, ensure the body contains valid HTML (e.g., `--body "<p>Hello <b>world</b></p>"`).
5. **Attachments.** Use `--attach FILE` (repeatable) with `send` or `draft` to attach files. Files must be < 3MB. To download attachments, first list them with `mail attachment list <ID>` to get attachment IDs, then download with `mail attachment download <MSG_ID> <ATT_ID>`.
6. **Confirm before outbound actions.** Show the user the exact recipient/subject/body (or event details) and get confirmation before running `mail send`, `mail reply`, or `cal create`. Prefer plain `reply` over `--reply-all` unless the user asks.
7. **Errors** go to stderr as `Error: ...` with exit code 1. `Message not found` / `Event not found` usually means a stale or mistyped ID — re-run the search.

## Typical Workflows

**Inbox triage**
```bash
outlook mail search --unread                 # 1. What's new
outlook mail read <ID>                       # 2. Read the relevant one
outlook mail reply <ID> --body "..."         # 3. Reply (after user confirms)
outlook mail reply <ID> --body "..." --draft # 3b. Or save as draft for review
outlook mail mark <ID>                       # 4. Mark handled
```

**"Am I free / what's coming up?"**
```bash
outlook cal list                             # This week
outlook cal list --start 2026-08-01 --end 2026-08-31
outlook cal read <ID>                        # Check attendees/agenda
```

**Schedule a meeting**
```bash
outlook cal create --subject "Sync" --start "2026-07-18 14:00" --end "2026-07-18 14:30" --location "Room A"
```

**Send HTML email**
```bash
outlook mail send --to user@example.com --subject "Report" --body "<h1>Q4 Report</h1><p>See attached.</p>" --html
outlook mail reply <ID> --body "<p>Thanks for the <strong>detailed</strong> update!</p>" --html
```

**Work with attachments**
```bash
outlook mail search --has-attachments               # Find messages with attachments
outlook mail attachment list <ID>                   # List attachments on a message
outlook mail attachment download <ID> <ATT_ID>      # Download to current directory
outlook mail attachment download <ID> <ATT_ID> -o ~/Downloads/report.pdf  # Custom path

outlook mail send --to user@example.com --subject "Files" --body "See attached" \
  --attach report.pdf --attach data.xlsx           # Send with multiple attachments
outlook mail draft --subject "WIP" --attach notes.txt  # Draft with attachment
```

# PR: Add Attachment Support for Email Commands

## Summary

This PR adds comprehensive attachment support across all email commands (`send`, `draft`, `reply`), enabling users to send emails with file attachments and manage attachments on received messages.

## Changes

### New Features

- **Attachment flag for email commands**: Added `--attach` flag (repeatable) to `mail send`, `mail draft`, and `mail reply` commands
- **Attachment management commands**: New `mail attachment` subcommand group with `list` and `download` commands
- **Smart HTML stripping**: When replying in plain text to an HTML email, the quoted original message is automatically converted to clean text (no HTML garbage)

### API Changes

| Function | Change |
|----------|--------|
| `CreateDraft()` | Now returns `(*DraftMessage, error)` with draft ID for attachment support |
| `Reply()` | Now returns `(*DraftMessage, error)` to enable adding attachments to replies |
| `SendDraft()` | **New** - sends an existing draft message |
| `ListAttachments()` | **New** - lists all attachments on a message |
| `GetAttachment()` | **New** - fetches attachment content (base64) |
| `AddAttachment()` | **New** - adds file attachment to a draft |

### New Types

```go
type Attachment struct {
    ID, Name, ContentType string
    Size                  int
    IsInline              bool
    ContentBytes          string // base64 encoded
}

type DraftMessage struct {
    ID, Subject string
}
```

### Files Changed

| File | Changes |
|------|---------|
| `cmd/mail.go` | Added `--attach` flag to send/draft/reply; new `attachment` subcommand with `list`/`download` |
| `internal/graph/mail.go` | Added attachment API methods, `DraftMessage` type, HTML stripping for text replies |
| `internal/graph/client_test.go` | Tests for attachment APIs |
| `cmd/cmd_test.go` | CLI integration tests for attachment commands |
| `internal/display/mail.go` | `AttachmentTable` display, attachment indicator in `MailDetail` |
| `README.md` | Updated documentation with attachment examples |
| `.claude/skills/outlook-cli/SKILL.md` | Updated skill with attachment commands |

## Usage Examples

```bash
# Send email with attachment
outlook mail send --to user@example.com --subject "Report" --body "See attached" \
  --attach report.pdf

# Send with multiple attachments
outlook mail send --to user@example.com --subject "Files" --body "Documents" \
  --attach doc1.pdf --attach doc2.xlsx

# Create draft with attachment
outlook mail draft --to user@example.com --subject "WIP" --body "Draft" \
  --attach notes.txt

# Reply with attachment
outlook mail reply MESSAGE_ID --body "Here's the file you requested" \
  --attach response.pdf

# Reply-all with attachment as draft
outlook mail reply MESSAGE_ID --body "FYI" --reply-all --draft \
  --attach summary.pdf

# List attachments on a message
outlook mail attachment list MESSAGE_ID

# Download attachment
outlook mail attachment download MESSAGE_ID ATTACHMENT_ID
outlook mail attachment download MESSAGE_ID ATTACHMENT_ID -o ~/Downloads/file.pdf
```

## Technical Notes

- **File size limit**: Attachments must be < 3MB (Microsoft Graph API limitation for direct upload)
- **Attachment flow**: When sending with attachments, the CLI creates a draft → adds attachments → sends the draft
- **MIME type detection**: Uses Go's `mime.TypeByExtension()`, defaults to `application/octet-stream`
- **HTML stripping**: Uses regex-based HTML tag removal with proper entity unescaping for clean text quotes

## Test Plan

- [x] `mail send --attach` creates and sends email with attachment
- [x] `mail draft --attach` creates draft with attachment
- [x] `mail reply --attach` sends reply with attachment
- [x] `mail attachment list` displays attachment table (Name, Size, Type, ID)
- [x] `mail attachment download` saves file with correct content
- [x] Attachment indicator (`*`) shows in mail search results
- [x] `mail read` shows "Attachments: Yes" for messages with attachments
- [x] Plain text reply to HTML email shows clean quoted text (no HTML tags)
- [x] All existing tests pass

---

Generated with [Claude Code](https://claude.ai/code)

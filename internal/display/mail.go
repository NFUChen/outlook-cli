package display

import (
	"fmt"
	"strings"

	"github.com/mhattingpete/outlook-cli/internal/graph"
)

const dateLayout = "2006-01-02 15:04"

func recipientString(r *graph.Recipient) string {
	if r == nil {
		return ""
	}
	return r.String()
}

// MailTable prints a table of messages.
func (p *Printer) MailTable(msgs []graph.Message) {
	headers := []string{" ", "Imp", "Att", "From", "Subject", "Date", "ID"}
	rows := make([][]string, 0, len(msgs))
	for _, m := range msgs {
		status := " "
		if !m.IsRead {
			status = "●"
		}
		imp := ""
		if strings.EqualFold(m.Importance, "high") {
			imp = "!"
		}
		att := ""
		if m.HasAttachments {
			att = "*"
		}
		date := ""
		if !m.ReceivedDateTime.IsZero() {
			date = m.ReceivedDateTime.Local().Format(dateLayout)
		}
		rows = append(rows, []string{
			status, imp, att,
			truncate(recipientString(m.From), 30),
			m.Subject, date, m.ID,
		})
	}
	p.table("Messages", headers, rows)
}

// MailDetail prints a single message with headers and body.
func (p *Printer) MailDetail(m *graph.Message) {
	sender := recipientString(m.From)
	if sender == "" {
		sender = "Unknown"
	}
	toParts := make([]string, 0, len(m.ToRecipients))
	for _, r := range m.ToRecipients {
		toParts = append(toParts, r.String())
	}
	ccParts := make([]string, 0, len(m.CcRecipients))
	for _, r := range m.CcRecipients {
		ccParts = append(ccParts, r.String())
	}
	date := ""
	if !m.ReceivedDateTime.IsZero() {
		date = m.ReceivedDateTime.Local().Format(dateLayout)
	}

	lines := []string{
		p.field("From", sender),
		p.field("To", strings.Join(toParts, ", ")),
	}
	if len(ccParts) > 0 {
		lines = append(lines, p.field("CC", strings.Join(ccParts, ", ")))
	}
	lines = append(lines, p.field("Date", date))
	if m.HasAttachments {
		lines = append(lines, p.field("Attachments", "Yes (use 'mail attachment list' to view)"))
	}

	body := "(empty)"
	if m.Body != nil && m.Body.Content != "" {
		body = m.Body.Content
	}
	if LooksLikeHTML(body) {
		body = StripHTML(body)
	}

	p.panel(m.Subject, lines)
	p.Println(body)
}

// AttachmentTable prints a table of attachments.
func (p *Printer) AttachmentTable(attachments []graph.Attachment) {
	headers := []string{"Name", "Size", "Type", "ID"}
	rows := make([][]string, 0, len(attachments))
	for _, a := range attachments {
		size := formatSize(a.Size)
		rows = append(rows, []string{a.Name, size, a.ContentType, a.ID})
	}
	p.table("Attachments", headers, rows)
}

// formatSize formats bytes into human-readable size.
func formatSize(bytes int) string {
	const (
		KB = 1024
		MB = KB * 1024
	)
	switch {
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

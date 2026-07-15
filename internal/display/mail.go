package display

import (
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

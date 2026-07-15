package display

import (
	"strings"

	"github.com/mhattingpete/outlook-cli/internal/graph"
)

func eventTime(d *graph.DateTimeTimeZone, allDay bool) string {
	t, ok := d.Time(allDay)
	if !ok {
		return ""
	}
	return t.Format(dateLayout)
}

func eventLocation(ev *graph.Event) string {
	if ev.Location == nil {
		return ""
	}
	return ev.Location.DisplayName
}

// EventTable prints a table of calendar events.
func (p *Printer) EventTable(events []graph.Event) {
	headers := []string{"Subject", "Start", "End", "Location", "Info", "ID"}
	rows := make([][]string, 0, len(events))
	for i := range events {
		ev := &events[i]
		var infoParts []string
		if ev.IsAllDay {
			infoParts = append(infoParts, "All-day")
		}
		if ev.IsRecurring() {
			infoParts = append(infoParts, "Recurring")
		}
		rows = append(rows, []string{
			ev.Subject,
			eventTime(ev.Start, ev.IsAllDay),
			eventTime(ev.End, ev.IsAllDay),
			truncate(eventLocation(ev), 25),
			strings.Join(infoParts, ", "),
			ev.ID,
		})
	}
	p.table("Events", headers, rows)
}

func formatAttendee(att graph.Attendee) string {
	label := att.EmailAddress.String()
	if label == "" {
		label = "?"
	}
	if att.Status.Response != "" {
		label += " (" + att.Status.Response + ")"
	}
	return label
}

// EventDetail prints a single event with headers and description.
func (p *Printer) EventDetail(ev *graph.Event) {
	lines := []string{
		p.field("Start", eventTime(ev.Start, ev.IsAllDay)),
		p.field("End", eventTime(ev.End, ev.IsAllDay)),
		p.field("Location", eventLocation(ev)),
	}
	if ev.Organizer != nil {
		lines = append(lines, p.field("Organizer", ev.Organizer.String()))
	}
	if ev.IsAllDay {
		lines = append(lines, p.field("All-day", "Yes"))
	}
	if ev.IsRecurring() {
		lines = append(lines, p.field("Recurring", "Yes"))
	}
	if len(ev.Attendees) > 0 {
		lines = append(lines, p.field("Attendees", ""))
		for _, att := range ev.Attendees {
			lines = append(lines, "  • "+formatAttendee(att))
		}
	}

	body := "(no description)"
	if ev.Body != nil && ev.Body.Content != "" {
		body = ev.Body.Content
	}
	if LooksLikeHTML(body) {
		body = StripHTML(body)
	}

	p.panel(ev.Subject, lines)
	p.Println(body)
}

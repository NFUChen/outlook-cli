package display

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mhattingpete/outlook-cli/internal/graph"
)

func testPrinter() (*Printer, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	return &Printer{Out: out, Err: errBuf, Color: false}, out, errBuf
}

func TestErrorMessage(t *testing.T) {
	p, _, errBuf := testPrinter()
	p.Error("something broke")
	got := errBuf.String()
	if !strings.Contains(got, "Error:") || !strings.Contains(got, "something broke") {
		t.Errorf("got %q", got)
	}
}

func TestSuccessMessage(t *testing.T) {
	p, out, _ := testPrinter()
	p.Success("it worked")
	got := out.String()
	if !strings.Contains(got, "OK:") || !strings.Contains(got, "it worked") {
		t.Errorf("got %q", got)
	}
}

func TestWarnMessage(t *testing.T) {
	p, out, _ := testPrinter()
	p.Warn("careful")
	got := out.String()
	if !strings.Contains(got, "Warning:") || !strings.Contains(got, "careful") {
		t.Errorf("got %q", got)
	}
}

func TestColorEmitsAnsi(t *testing.T) {
	out := &bytes.Buffer{}
	p := &Printer{Out: out, Err: out, Color: true}
	p.Error("boom")
	if !strings.Contains(out.String(), "\x1b[") {
		t.Error("expected ANSI codes with Color enabled")
	}
}

func TestNoColorPlainText(t *testing.T) {
	p, out, errBuf := testPrinter()
	p.Error("boom")
	p.Success("yay")
	if strings.Contains(out.String()+errBuf.String(), "\x1b[") {
		t.Error("unexpected ANSI codes with Color disabled")
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 30); got != "short" {
		t.Errorf("got %q", got)
	}
	long := strings.Repeat("a", 40)
	got := truncate(long, 30)
	if len([]rune(got)) != 30 {
		t.Errorf("truncated length = %d, want 30", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("got %q, want … suffix", got)
	}
}

func sampleMessage() graph.Message {
	return graph.Message{
		ID:      "msg-123",
		Subject: "Test Subject",
		From: &graph.Recipient{
			EmailAddress: graph.EmailAddress{Name: "Alice", Address: "alice@example.com"},
		},
		ToRecipients: []graph.Recipient{
			{EmailAddress: graph.EmailAddress{Name: "Bob", Address: "bob@example.com"}},
		},
		ReceivedDateTime: time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
		IsRead:           false,
		Importance:       "high",
		HasAttachments:   true,
		Body:             &graph.ItemBody{ContentType: "Text", Content: "Hello, world!"},
	}
}

func TestMailTable(t *testing.T) {
	p, out, _ := testPrinter()
	p.MailTable([]graph.Message{sampleMessage()})
	got := out.String()
	for _, want := range []string{"Messages", "Test Subject", "msg-123", "●", "!", "*", "Alice <alice@example.com>"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestMailTableEmpty(t *testing.T) {
	p, out, _ := testPrinter()
	p.MailTable(nil)
	if !strings.Contains(out.String(), "Messages") {
		t.Errorf("got %q", out.String())
	}
}

func TestMailDetail(t *testing.T) {
	p, out, _ := testPrinter()
	msg := sampleMessage()
	p.MailDetail(&msg)
	got := out.String()
	for _, want := range []string{"Test Subject", "Hello, world!", "From:", "To:", "Bob <bob@example.com>"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "CC:") {
		t.Error("CC line should be omitted when empty")
	}
}

func TestMailDetailStripsHTML(t *testing.T) {
	p, out, _ := testPrinter()
	msg := sampleMessage()
	msg.Body = &graph.ItemBody{ContentType: "HTML", Content: "<html><p>rich&nbsp;body</p></html>"}
	p.MailDetail(&msg)
	got := out.String()
	if !strings.Contains(got, "rich body") {
		t.Errorf("HTML not stripped:\n%s", got)
	}
	if strings.Contains(got, "<p>") {
		t.Error("raw HTML leaked into output")
	}
}

func sampleEvent() graph.Event {
	return graph.Event{
		ID:      "ev-1",
		Subject: "Team Meeting",
		Start:   &graph.DateTimeTimeZone{DateTime: "2026-07-15T09:00:00.0000000", TimeZone: "UTC"},
		End:     &graph.DateTimeTimeZone{DateTime: "2026-07-15T10:00:00.0000000", TimeZone: "UTC"},
		Location: &graph.Location{
			DisplayName: "Room 1",
		},
		Organizer: &graph.Recipient{
			EmailAddress: graph.EmailAddress{Name: "Carol", Address: "carol@example.com"},
		},
		IsAllDay:   false,
		Recurrence: []byte(`{"pattern":{"type":"weekly"}}`),
		Body:       &graph.ItemBody{ContentType: "Text", Content: "Weekly sync"},
	}
}

func TestEventTable(t *testing.T) {
	p, out, _ := testPrinter()
	p.EventTable([]graph.Event{sampleEvent()})
	got := out.String()
	for _, want := range []string{"Events", "Team Meeting", "Room 1", "Recurring", "ev-1"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestEventDetail(t *testing.T) {
	p, out, _ := testPrinter()
	ev := sampleEvent()
	ev.Attendees = []graph.Attendee{
		{EmailAddress: graph.EmailAddress{Name: "Dave", Address: "dave@example.com"}},
	}
	ev.Attendees[0].Status.Response = "accepted"
	p.EventDetail(&ev)
	got := out.String()
	for _, want := range []string{
		"Team Meeting", "Weekly sync", "Start:", "End:",
		"Carol <carol@example.com>", "Recurring:",
		"Dave <dave@example.com> (accepted)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestEventDetailAllDay(t *testing.T) {
	p, out, _ := testPrinter()
	ev := sampleEvent()
	ev.IsAllDay = true
	ev.Recurrence = nil
	p.EventDetail(&ev)
	got := out.String()
	if !strings.Contains(got, "All-day: Yes") {
		t.Errorf("missing all-day marker:\n%s", got)
	}
	if strings.Contains(got, "Recurring:") {
		t.Error("Recurring line should be omitted")
	}
	// All-day events keep the wall-clock date (no timezone conversion).
	if !strings.Contains(got, "2026-07-15 09:00") {
		t.Errorf("all-day start not kept as wall clock:\n%s", got)
	}
}

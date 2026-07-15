package graph

import (
	"net/url"
	"testing"
	"time"
)

func TestMailQueryDefaults(t *testing.T) {
	v := MailQuery{Limit: 25}.Values()
	if got := v.Get("$top"); got != "25" {
		t.Errorf("$top = %q, want 25", got)
	}
	if got := v.Get("$select"); got != mailSelect {
		t.Errorf("$select = %q", got)
	}
	if got := v.Get("$orderby"); got != "receivedDateTime desc" {
		t.Errorf("$orderby = %q", got)
	}
	if v.Has("$filter") {
		t.Errorf("unexpected $filter: %q", v.Get("$filter"))
	}
	if v.Has("$search") {
		t.Errorf("unexpected $search: %q", v.Get("$search"))
	}
}

func TestMailQuerySearchExcludesFilterAndOrderby(t *testing.T) {
	q := MailQuery{Search: "hello world", Unread: true, Sender: "a@b.c", Limit: 10}
	v := q.Values()
	if got := v.Get("$search"); got != `"hello world"` {
		t.Errorf("$search = %q", got)
	}
	if v.Has("$filter") {
		t.Errorf("$filter must be omitted with $search, got %q", v.Get("$filter"))
	}
	if v.Has("$orderby") {
		t.Errorf("$orderby must be omitted with $search, got %q", v.Get("$orderby"))
	}
}

func TestMailQuerySearchEscapesQuotes(t *testing.T) {
	v := MailQuery{Search: `say "hi"`, Limit: 5}.Values()
	if got := v.Get("$search"); got != `"say \"hi\""` {
		t.Errorf("$search = %q", got)
	}
}

func TestMailQueryFilters(t *testing.T) {
	tests := []struct {
		name string
		q    MailQuery
		want string
	}{
		{
			name: "start date",
			q:    MailQuery{Start: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)},
			want: "receivedDateTime ge 2024-01-02T00:00:00Z",
		},
		{
			name: "end date",
			q:    MailQuery{End: time.Date(2024, 3, 4, 0, 0, 0, 0, time.UTC)},
			want: "receivedDateTime le 2024-03-04T00:00:00Z",
		},
		{
			name: "unread",
			q:    MailQuery{Unread: true},
			want: "isRead eq false",
		},
		{
			name: "important",
			q:    MailQuery{Important: true},
			want: "importance eq 'high'",
		},
		{
			name: "has attachments",
			q:    MailQuery{HasAttachments: true},
			want: "hasAttachments eq true",
		},
		{
			name: "sender",
			q:    MailQuery{Sender: "alice@example.com"},
			want: "contains(from/emailAddress/address,'alice@example.com')",
		},
		{
			name: "sender escapes single quotes",
			q:    MailQuery{Sender: "o'brien"},
			want: "contains(from/emailAddress/address,'o''brien')",
		},
		{
			name: "combined",
			q: MailQuery{
				Start:  time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
				Unread: true,
				Sender: "a@b.c",
			},
			want: "receivedDateTime ge 2024-01-02T00:00:00Z and isRead eq false and contains(from/emailAddress/address,'a@b.c')",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.q.Values().Get("$filter"); got != tt.want {
				t.Errorf("$filter = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEventQueryValues(t *testing.T) {
	q := EventQuery{
		Start:     time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC),
		End:       time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC),
		Subject:   "sync",
		Location:  "Room 1",
		Organizer: "boss@example.com",
		AllDay:    true,
		Recurring: true,
		Limit:     25,
	}
	v := q.Values()

	want := "start/dateTime ge '2026-07-15T00:00:00'" +
		" and end/dateTime le '2026-07-22T00:00:00'" +
		" and contains(subject,'sync')" +
		" and contains(location/displayName,'Room 1')" +
		" and contains(organizer/emailAddress/address,'boss@example.com')" +
		" and isAllDay eq true" +
		" and recurrence ne null"
	if got := v.Get("$filter"); got != want {
		t.Errorf("$filter = %q, want %q", got, want)
	}
	if got := v.Get("$orderby"); got != "start/dateTime" {
		t.Errorf("$orderby = %q", got)
	}
	if got := v.Get("$top"); got != "25" {
		t.Errorf("$top = %q", got)
	}
}

func TestEventQueryNoFilters(t *testing.T) {
	v := EventQuery{Limit: 10}.Values()
	if v.Has("$filter") {
		t.Errorf("unexpected $filter: %q", v.Get("$filter"))
	}
}

func TestQueryValuesEncode(t *testing.T) {
	// Sanity check: encoding produces valid URL query strings.
	v := MailQuery{Search: "budget report", Limit: 5}.Values()
	parsed, err := url.ParseQuery(v.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.Get("$search"); got != `"budget report"` {
		t.Errorf("round-trip $search = %q", got)
	}
}

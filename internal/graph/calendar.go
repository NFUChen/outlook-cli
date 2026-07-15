package graph

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DateTimeTimeZone is a Graph dateTimeTimeZone object.
type DateTimeTimeZone struct {
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone"`
}

// Time parses the dateTime value. UTC values are converted to local time
// unless keepWallClock is true (e.g. all-day events).
func (d *DateTimeTimeZone) Time(keepWallClock bool) (time.Time, bool) {
	if d == nil || d.DateTime == "" {
		return time.Time{}, false
	}
	s := d.DateTime
	if i := strings.Index(s, "."); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSuffix(s, "Z")
	t, err := time.Parse("2006-01-02T15:04:05", s)
	if err != nil {
		return time.Time{}, false
	}
	if keepWallClock || !strings.EqualFold(d.TimeZone, "UTC") {
		return t, true
	}
	return t.UTC().Local(), true
}

// Location is a Graph location object.
type Location struct {
	DisplayName string `json:"displayName"`
}

// Attendee is a Graph attendee object.
type Attendee struct {
	EmailAddress EmailAddress `json:"emailAddress"`
	Status       struct {
		Response string `json:"response"`
	} `json:"status"`
}

// Event is a Graph event.
type Event struct {
	ID         string            `json:"id"`
	Subject    string            `json:"subject"`
	Start      *DateTimeTimeZone `json:"start"`
	End        *DateTimeTimeZone `json:"end"`
	Location   *Location         `json:"location"`
	Organizer  *Recipient        `json:"organizer"`
	Attendees  []Attendee        `json:"attendees"`
	IsAllDay   bool              `json:"isAllDay"`
	Recurrence json.RawMessage   `json:"recurrence"`
	Body       *ItemBody         `json:"body"`
}

// IsRecurring reports whether the event has a recurrence pattern.
func (e *Event) IsRecurring() bool {
	return len(e.Recurrence) > 0 && string(e.Recurrence) != "null"
}

// ListEvents lists events on the default calendar matching the query.
func (c *Client) ListEvents(ctx context.Context, q EventQuery) ([]Event, error) {
	var out listResponse[Event]
	if err := c.do(ctx, http.MethodGet, "/me/calendar/events", q.Values(), nil, &out); err != nil {
		return nil, err
	}
	return out.Value, nil
}

// GetEvent fetches a single event by ID.
func (c *Client) GetEvent(ctx context.Context, id string) (*Event, error) {
	var ev Event
	if err := c.do(ctx, http.MethodGet, "/me/calendar/events/"+url.PathEscape(id), nil, nil, &ev); err != nil {
		return nil, err
	}
	return &ev, nil
}

// NewEvent describes a calendar event to create.
type NewEvent struct {
	Subject  string
	Start    time.Time
	End      time.Time
	Body     string
	Location string
}

// CreateEvent creates an event on the default calendar. Times are sent in UTC.
func (c *Client) CreateEvent(ctx context.Context, e NewEvent) error {
	const layout = "2006-01-02T15:04:05"
	payload := map[string]any{
		"subject": e.Subject,
		"start":   DateTimeTimeZone{DateTime: e.Start.UTC().Format(layout), TimeZone: "UTC"},
		"end":     DateTimeTimeZone{DateTime: e.End.UTC().Format(layout), TimeZone: "UTC"},
	}
	if e.Body != "" {
		payload["body"] = ItemBody{ContentType: "Text", Content: e.Body}
	}
	if e.Location != "" {
		payload["location"] = Location{DisplayName: e.Location}
	}
	return c.do(ctx, http.MethodPost, "/me/calendar/events", nil, payload, nil)
}

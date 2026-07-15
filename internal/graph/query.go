package graph

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

// escapeODataString escapes single quotes for OData string literals.
func escapeODataString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

const mailSelect = "id,subject,from,receivedDateTime,isRead,importance,hasAttachments"

// MailQuery describes filters for listing messages.
type MailQuery struct {
	Search         string
	Sender         string
	Start          time.Time
	End            time.Time
	Unread         bool
	Important      bool
	HasAttachments bool
	Limit          int
}

// Values builds the Graph query parameters. When Search is set, all other
// filters and ordering are omitted (Graph cannot combine $search with $filter).
func (q MailQuery) Values() url.Values {
	v := url.Values{}
	v.Set("$top", strconv.Itoa(q.Limit))
	v.Set("$select", mailSelect)

	if q.Search != "" {
		v.Set("$search", `"`+strings.ReplaceAll(q.Search, `"`, `\"`)+`"`)
		return v
	}

	var filters []string
	if !q.Start.IsZero() {
		filters = append(filters, "receivedDateTime ge "+q.Start.UTC().Format("2006-01-02T15:04:05Z"))
	}
	if !q.End.IsZero() {
		filters = append(filters, "receivedDateTime le "+q.End.UTC().Format("2006-01-02T15:04:05Z"))
	}
	if q.Unread {
		filters = append(filters, "isRead eq false")
	}
	if q.Important {
		filters = append(filters, "importance eq 'high'")
	}
	if q.HasAttachments {
		filters = append(filters, "hasAttachments eq true")
	}
	if q.Sender != "" {
		filters = append(filters, "contains(from/emailAddress/address,'"+escapeODataString(q.Sender)+"')")
	}
	if len(filters) > 0 {
		v.Set("$filter", strings.Join(filters, " and "))
	}
	v.Set("$orderby", "receivedDateTime desc")
	return v
}

const eventSelect = "id,subject,start,end,location,isAllDay,recurrence"

// EventQuery describes filters for listing calendar events.
type EventQuery struct {
	Start     time.Time
	End       time.Time
	Subject   string
	Location  string
	Organizer string
	AllDay    bool
	Recurring bool
	Limit     int
}

// Values builds the Graph query parameters for listing events.
func (q EventQuery) Values() url.Values {
	v := url.Values{}
	v.Set("$top", strconv.Itoa(q.Limit))
	v.Set("$select", eventSelect)

	var filters []string
	if !q.Start.IsZero() {
		filters = append(filters, "start/dateTime ge '"+q.Start.UTC().Format("2006-01-02T15:04:05")+"'")
	}
	if !q.End.IsZero() {
		filters = append(filters, "end/dateTime le '"+q.End.UTC().Format("2006-01-02T15:04:05")+"'")
	}
	if q.Subject != "" {
		filters = append(filters, "contains(subject,'"+escapeODataString(q.Subject)+"')")
	}
	if q.Location != "" {
		filters = append(filters, "contains(location/displayName,'"+escapeODataString(q.Location)+"')")
	}
	if q.Organizer != "" {
		filters = append(filters, "contains(organizer/emailAddress/address,'"+escapeODataString(q.Organizer)+"')")
	}
	if q.AllDay {
		filters = append(filters, "isAllDay eq true")
	}
	if q.Recurring {
		filters = append(filters, "recurrence ne null")
	}
	if len(filters) > 0 {
		v.Set("$filter", strings.Join(filters, " and "))
	}
	v.Set("$orderby", "start/dateTime")
	return v
}

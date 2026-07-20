package graph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func testClient(srv *httptest.Server) *Client {
	return &Client{
		HTTP:    srv.Client(),
		BaseURL: srv.URL,
		Tokens:  oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}),
	}
}

func TestClientAuthorizationHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"value":[]}`)
	}))
	defer srv.Close()

	if _, err := testClient(srv).ListMessages(context.Background(), "inbox", MailQuery{Limit: 5}); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q", gotAuth)
	}
}

func TestResolveFolderInbox(t *testing.T) {
	c := &Client{} // must not make any HTTP calls
	id, err := c.ResolveFolder(context.Background(), "Inbox")
	if err != nil {
		t.Fatal(err)
	}
	if id != "inbox" {
		t.Errorf("id = %q, want inbox", id)
	}
}

func TestResolveFolderByName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/me/mailFolders" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("$filter"); got != "displayName eq 'Archive'" {
			t.Errorf("$filter = %q", got)
		}
		fmt.Fprint(w, `{"value":[{"id":"folder-42","displayName":"Archive"}]}`)
	}))
	defer srv.Close()

	id, err := testClient(srv).ResolveFolder(context.Background(), "Archive")
	if err != nil {
		t.Fatal(err)
	}
	if id != "folder-42" {
		t.Errorf("id = %q", id)
	}
}

func TestResolveFolderNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"value":[]}`)
	}))
	defer srv.Close()

	_, err := testClient(srv).ResolveFolder(context.Background(), "Nope")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestListMessagesRequestAndDecode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q", r.Method)
		}
		if r.URL.Path != "/me/mailFolders/inbox/messages" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("$top"); got != "10" {
			t.Errorf("$top = %q", got)
		}
		fmt.Fprint(w, `{"value":[{
			"id":"msg-1","subject":"Hello",
			"from":{"emailAddress":{"name":"Alice","address":"alice@example.com"}},
			"receivedDateTime":"2026-07-01T12:00:00Z",
			"isRead":false,"importance":"high","hasAttachments":true
		}]}`)
	}))
	defer srv.Close()

	msgs, err := testClient(srv).ListMessages(context.Background(), "inbox", MailQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages", len(msgs))
	}
	m := msgs[0]
	if m.ID != "msg-1" || m.Subject != "Hello" || m.IsRead || !m.HasAttachments {
		t.Errorf("unexpected message: %+v", m)
	}
	if m.From.EmailAddress.Address != "alice@example.com" {
		t.Errorf("from = %+v", m.From)
	}
	if m.ReceivedDateTime.IsZero() {
		t.Error("receivedDateTime not parsed")
	}
}

func TestGetMessageNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":{"code":"ErrorItemNotFound","message":"not found"}}`)
	}))
	defer srv.Close()

	_, err := testClient(srv).GetMessage(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestSendMailRequest(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/me/sendMail" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	err := testClient(srv).SendMail(context.Background(), "to@x.com", "cc@x.com", "Subj", "Body", "")
	if err != nil {
		t.Fatal(err)
	}
	msg := gotBody["message"].(map[string]any)
	if msg["subject"] != "Subj" {
		t.Errorf("subject = %v", msg["subject"])
	}
	if _, ok := msg["ccRecipients"]; !ok {
		t.Error("ccRecipients missing")
	}
}

func TestSendMailOmitsEmptyCc(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	if err := testClient(srv).SendMail(context.Background(), "to@x.com", "", "S", "B", ""); err != nil {
		t.Fatal(err)
	}
	msg := gotBody["message"].(map[string]any)
	if _, ok := msg["ccRecipients"]; ok {
		t.Error("ccRecipients should be omitted when empty")
	}
}

func TestSendMailWithHTMLContentType(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	err := testClient(srv).SendMail(context.Background(), "to@x.com", "", "Subj", "<h1>HTML</h1>", "HTML")
	if err != nil {
		t.Fatal(err)
	}
	msg := gotBody["message"].(map[string]any)
	body := msg["body"].(map[string]any)
	if body["contentType"] != "HTML" {
		t.Errorf("contentType = %v, want HTML", body["contentType"])
	}
	if body["content"] != "<h1>HTML</h1>" {
		t.Errorf("content = %v", body["content"])
	}
}

func TestSendMailDefaultsToText(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	err := testClient(srv).SendMail(context.Background(), "to@x.com", "", "Subj", "Plain text", "")
	if err != nil {
		t.Fatal(err)
	}
	msg := gotBody["message"].(map[string]any)
	body := msg["body"].(map[string]any)
	if body["contentType"] != "Text" {
		t.Errorf("contentType = %v, want Text", body["contentType"])
	}
}

func TestCreateDraftRequest(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/me/messages" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"draft-1"}`)
	}))
	defer srv.Close()

	err := testClient(srv).CreateDraft(context.Background(), Draft{
		To:         []string{"alice@example.com", "bob@example.com"},
		Cc:         []string{"carol@example.com"},
		Bcc:        []string{"dave@example.com"},
		Subject:    "Quarterly report",
		Body:       "Draft body text",
		Importance: "high",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotBody["subject"] != "Quarterly report" {
		t.Errorf("subject = %v", gotBody["subject"])
	}
	if gotBody["importance"] != "high" {
		t.Errorf("importance = %v", gotBody["importance"])
	}
	to := gotBody["toRecipients"].([]any)
	if len(to) != 2 {
		t.Errorf("toRecipients = %v", to)
	}
	if _, ok := gotBody["ccRecipients"]; !ok {
		t.Error("ccRecipients missing")
	}
	if _, ok := gotBody["bccRecipients"]; !ok {
		t.Error("bccRecipients missing")
	}
	body := gotBody["body"].(map[string]any)
	if body["content"] != "Draft body text" {
		t.Errorf("body = %v", body)
	}
}

func TestCreateDraftOmitsEmptyFields(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"draft-1"}`)
	}))
	defer srv.Close()

	if err := testClient(srv).CreateDraft(context.Background(), Draft{Subject: "Ideas"}); err != nil {
		t.Fatal(err)
	}
	if len(gotBody) != 1 || gotBody["subject"] != "Ideas" {
		t.Errorf("body = %v, want only subject", gotBody)
	}
}

func TestCreateDraftWithHTMLContentType(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"draft-1"}`)
	}))
	defer srv.Close()

	err := testClient(srv).CreateDraft(context.Background(), Draft{
		To:          []string{"alice@example.com"},
		Subject:     "HTML Draft",
		Body:        "<p>Draft content</p>",
		ContentType: "HTML",
	})
	if err != nil {
		t.Fatal(err)
	}
	body := gotBody["body"].(map[string]any)
	if body["contentType"] != "HTML" {
		t.Errorf("contentType = %v, want HTML", body["contentType"])
	}
	if body["content"] != "<p>Draft content</p>" {
		t.Errorf("content = %v", body["content"])
	}
}

func TestReplyEndpoints(t *testing.T) {
	tests := []struct {
		all      bool
		wantPath string
	}{
		{false, "/me/messages/msg-1/reply"},
		{true, "/me/messages/msg-1/replyAll"},
	}
	for _, tt := range tests {
		var gotPath string
		var gotBody map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Handle GET request to fetch original message
			if r.Method == http.MethodGet && r.URL.Path == "/me/messages/msg-1" {
				fmt.Fprint(w, `{
					"id":"msg-1","subject":"Original",
					"from":{"emailAddress":{"name":"Alice","address":"alice@example.com"}},
					"toRecipients":[{"emailAddress":{"address":"bob@example.com"}}],
					"receivedDateTime":"2026-07-01T12:00:00Z",
					"body":{"contentType":"Text","content":"Original message"}
				}`)
				return
			}
			// Handle POST request to create reply
			gotPath = r.URL.Path
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusAccepted)
		}))

		if err := testClient(srv).Reply(context.Background(), "msg-1", "thanks", "", tt.all, false); err != nil {
			t.Fatal(err)
		}
		if gotPath != tt.wantPath {
			t.Errorf("path = %q, want %q", gotPath, tt.wantPath)
		}
		msg := gotBody["message"].(map[string]any)
		body := msg["body"].(map[string]any)
		content := body["content"].(string)
		if !strings.Contains(content, "thanks") {
			t.Errorf("body.content missing 'thanks': %q", content)
		}
		if !strings.Contains(content, "Original message") {
			t.Errorf("body.content missing quoted original: %q", content)
		}
		srv.Close()
	}
}

func TestReplyWithHTMLContentType(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle GET request to fetch original message
		if r.Method == http.MethodGet && r.URL.Path == "/me/messages/msg-1" {
			fmt.Fprint(w, `{
				"id":"msg-1","subject":"Test",
				"from":{"emailAddress":{"name":"Alice","address":"alice@example.com"}},
				"toRecipients":[{"emailAddress":{"address":"bob@example.com"}}],
				"receivedDateTime":"2026-07-01T12:00:00Z",
				"body":{"contentType":"HTML","content":"<p>Original HTML</p>"}
			}`)
			return
		}
		// Handle POST request
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	err := testClient(srv).Reply(context.Background(), "msg-1", "<b>HTML reply</b>", "HTML", false, false)
	if err != nil {
		t.Fatal(err)
	}
	msg := gotBody["message"].(map[string]any)
	body := msg["body"].(map[string]any)
	if body["contentType"] != "HTML" {
		t.Errorf("contentType = %v, want HTML", body["contentType"])
	}
	content := body["content"].(string)
	if !strings.Contains(content, "<b>HTML reply</b>") {
		t.Errorf("content missing reply: %v", content)
	}
	if !strings.Contains(content, "Original HTML") {
		t.Errorf("content missing quoted original: %v", content)
	}
}

func TestReplyDefaultsToText(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle GET request to fetch original message
		if r.Method == http.MethodGet && r.URL.Path == "/me/messages/msg-1" {
			fmt.Fprint(w, `{
				"id":"msg-1","subject":"Test",
				"from":{"emailAddress":{"name":"Alice","address":"alice@example.com"}},
				"toRecipients":[{"emailAddress":{"address":"bob@example.com"}}],
				"receivedDateTime":"2026-07-01T12:00:00Z",
				"body":{"contentType":"Text","content":"Original text"}
			}`)
			return
		}
		// Handle POST request
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	err := testClient(srv).Reply(context.Background(), "msg-1", "Plain reply", "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	msg := gotBody["message"].(map[string]any)
	body := msg["body"].(map[string]any)
	if body["contentType"] != "Text" {
		t.Errorf("contentType = %v, want Text", body["contentType"])
	}
	content := body["content"].(string)
	if !strings.Contains(content, "Plain reply") {
		t.Errorf("content missing reply: %v", content)
	}
	if !strings.Contains(content, "Original text") {
		t.Errorf("content missing quoted original: %v", content)
	}
}

func TestReplyDraftEndpoints(t *testing.T) {
	tests := []struct {
		all      bool
		draft    bool
		wantPath string
	}{
		{false, true, "/me/messages/msg-1/createReply"},
		{true, true, "/me/messages/msg-1/createReplyAll"},
	}
	for _, tt := range tests {
		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Handle GET request to fetch original message
			if r.Method == http.MethodGet && r.URL.Path == "/me/messages/msg-1" {
				fmt.Fprint(w, `{
					"id":"msg-1","subject":"Test",
					"from":{"emailAddress":{"name":"Alice","address":"alice@example.com"}},
					"toRecipients":[{"emailAddress":{"address":"bob@example.com"}}],
					"receivedDateTime":"2026-07-01T12:00:00Z",
					"body":{"contentType":"Text","content":"Original"}
				}`)
				return
			}
			// Handle POST request
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusAccepted)
		}))

		if err := testClient(srv).Reply(context.Background(), "msg-1", "draft reply", "", tt.all, tt.draft); err != nil {
			t.Fatal(err)
		}
		if gotPath != tt.wantPath {
			t.Errorf("path = %q, want %q", gotPath, tt.wantPath)
		}
		srv.Close()
	}
}

func TestSetRead(t *testing.T) {
	var gotMethod string
	var gotBody map[string]bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	if err := testClient(srv).SetRead(context.Background(), "msg-1", false); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q", gotMethod)
	}
	if gotBody["isRead"] != false {
		t.Errorf("isRead = %v", gotBody["isRead"])
	}
}

func TestAPIErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"code":"InefficientFilter","message":"The restriction is too complex."}}`)
	}))
	defer srv.Close()

	_, err := testClient(srv).ListMessages(context.Background(), "inbox", MailQuery{Limit: 5})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.Status != 400 || apiErr.Code != "InefficientFilter" {
		t.Errorf("apiErr = %+v", apiErr)
	}
	if apiErr.Message != "The restriction is too complex." {
		t.Errorf("message = %q", apiErr.Message)
	}
}

func TestListEventsRequestAndDecode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/me/calendar/events" {
			t.Errorf("path = %q", r.URL.Path)
		}
		fmt.Fprint(w, `{"value":[{
			"id":"ev-1","subject":"Team Meeting",
			"start":{"dateTime":"2026-07-15T09:00:00.0000000","timeZone":"UTC"},
			"end":{"dateTime":"2026-07-15T10:00:00.0000000","timeZone":"UTC"},
			"location":{"displayName":"Room 1"},
			"isAllDay":false,
			"recurrence":{"pattern":{"type":"weekly"}}
		}]}`)
	}))
	defer srv.Close()

	events, err := testClient(srv).ListEvents(context.Background(), EventQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events", len(events))
	}
	ev := events[0]
	if ev.Subject != "Team Meeting" || ev.Location.DisplayName != "Room 1" {
		t.Errorf("unexpected event: %+v", ev)
	}
	if !ev.IsRecurring() {
		t.Error("expected recurring")
	}
}

func TestEventNotRecurringWhenNull(t *testing.T) {
	var ev Event
	if err := json.Unmarshal([]byte(`{"id":"e","recurrence":null}`), &ev); err != nil {
		t.Fatal(err)
	}
	if ev.IsRecurring() {
		t.Error("null recurrence must not be recurring")
	}
}

func TestCreateEventRequest(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/me/calendar/events" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"new-ev"}`)
	}))
	defer srv.Close()

	err := testClient(srv).CreateEvent(context.Background(), NewEvent{
		Subject:  "Standup",
		Start:    mustTimeUTC(t, "2026-07-16T09:00:00Z"),
		End:      mustTimeUTC(t, "2026-07-16T09:15:00Z"),
		Location: "Zoom",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotBody["subject"] != "Standup" {
		t.Errorf("subject = %v", gotBody["subject"])
	}
	start := gotBody["start"].(map[string]any)
	if start["dateTime"] != "2026-07-16T09:00:00" || start["timeZone"] != "UTC" {
		t.Errorf("start = %v", start)
	}
	if _, ok := gotBody["body"]; ok {
		t.Error("body should be omitted when empty")
	}
	loc := gotBody["location"].(map[string]any)
	if loc["displayName"] != "Zoom" {
		t.Errorf("location = %v", loc)
	}
}

func TestDateTimeTimeZoneParsing(t *testing.T) {
	d := &DateTimeTimeZone{DateTime: "2026-07-15T09:30:00.0000000", TimeZone: "UTC"}
	got, ok := d.Time(true) // keep wall clock
	if !ok {
		t.Fatal("parse failed")
	}
	if got.Hour() != 9 || got.Minute() != 30 {
		t.Errorf("got %v", got)
	}

	var nilDT *DateTimeTimeZone
	if _, ok := nilDT.Time(false); ok {
		t.Error("nil should not parse")
	}
}

func mustTimeUTC(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return tm
}

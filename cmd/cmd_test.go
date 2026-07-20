package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/mhattingpete/outlook-cli/internal/display"
	"github.com/mhattingpete/outlook-cli/internal/graph"
)

// testApp returns an App with a Graph client pointed at srv and captured output.
func testApp(srv *httptest.Server) (*App, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	app := &App{
		Printer: &display.Printer{Out: out, Err: errBuf, Color: false},
		NewClient: func() (*graph.Client, error) {
			c := &graph.Client{
				BaseURL: "http://invalid.test",
				Tokens:  oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}),
			}
			if srv != nil {
				c.BaseURL = srv.URL
				c.HTTP = srv.Client()
			}
			return c, nil
		},
		Authenticate:    func(ctx context.Context, clientID, tenantID string, w io.Writer) error { return nil },
		IsAuthenticated: func() bool { return true },
		Now:             func() time.Time { return time.Date(2026, 7, 15, 10, 30, 0, 0, time.Local) },
	}
	return app, out, errBuf
}

func run(app *App, args ...string) error {
	root := NewRoot(app, "test")
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	return root.Execute()
}

func TestVersionFlag(t *testing.T) {
	app, _, _ := testApp(nil)
	root := NewRoot(app, "1.2.3")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"--version"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "outlook-cli 1.2.3\n" {
		t.Errorf("got %q", got)
	}
}

// ── mail search ────────────────────────────────────────────────

const sampleMessagesJSON = `{"value":[{
	"id":"msg-1","subject":"Budget Report",
	"from":{"emailAddress":{"name":"Alice","address":"alice@example.com"}},
	"receivedDateTime":"2026-07-01T12:00:00Z",
	"isRead":false,"importance":"normal","hasAttachments":false
}]}`

func TestMailSearchDefault(t *testing.T) {
	var gotReq *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r.Clone(context.Background())
		fmt.Fprint(w, sampleMessagesJSON)
	}))
	defer srv.Close()

	app, out, _ := testApp(srv)
	if err := run(app, "mail", "search"); err != nil {
		t.Fatal(err)
	}
	if gotReq.URL.Path != "/me/mailFolders/inbox/messages" {
		t.Errorf("path = %q", gotReq.URL.Path)
	}
	q := gotReq.URL.Query()
	if q.Get("$top") != "25" {
		t.Errorf("$top = %q", q.Get("$top"))
	}
	if q.Has("$filter") || q.Has("$search") {
		t.Errorf("unexpected filter/search: %v", q)
	}
	if !strings.Contains(out.String(), "Budget Report") {
		t.Errorf("output missing subject:\n%s", out.String())
	}
}

func TestMailSearchQueryWithFiltersWarns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("$search"); got != `"budget"` {
			t.Errorf("$search = %q", got)
		}
		if q.Has("$filter") {
			t.Errorf("filter must be ignored with search: %q", q.Get("$filter"))
		}
		fmt.Fprint(w, sampleMessagesJSON)
	}))
	defer srv.Close()

	app, out, _ := testApp(srv)
	if err := run(app, "mail", "search", "budget", "--unread"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Warning: Filters are ignored when using text search.") {
		t.Errorf("missing warning:\n%s", out.String())
	}
}

func TestMailSearchFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filter := r.URL.Query().Get("$filter")
		for _, want := range []string{
			"isRead eq false",
			"importance eq 'high'",
			"contains(from/emailAddress/address,'alice')",
		} {
			if !strings.Contains(filter, want) {
				t.Errorf("$filter %q missing %q", filter, want)
			}
		}
		fmt.Fprint(w, sampleMessagesJSON)
	}))
	defer srv.Close()

	app, _, _ := testApp(srv)
	if err := run(app, "mail", "search", "--unread", "--important", "--from", "alice"); err != nil {
		t.Fatal(err)
	}
}

func TestMailSearchSenderAlias(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if filter := r.URL.Query().Get("$filter"); !strings.Contains(filter, "'bob'") {
			t.Errorf("$filter = %q", filter)
		}
		fmt.Fprint(w, sampleMessagesJSON)
	}))
	defer srv.Close()

	app, _, _ := testApp(srv)
	if err := run(app, "mail", "search", "--sender", "bob"); err != nil {
		t.Fatal(err)
	}
}

func TestMailSearchInvalidDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, sampleMessagesJSON)
	}))
	defer srv.Close()

	app, _, _ := testApp(srv)
	err := run(app, "mail", "search", "--start-date", "2024-13-99")
	if err == nil || err.Error() != "Invalid date format: 2024-13-99 (expected YYYY-MM-DD)" {
		t.Errorf("err = %v", err)
	}
}

func TestMailSearchFolderNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/me/mailFolders" {
			t.Errorf("path = %q", r.URL.Path)
		}
		fmt.Fprint(w, `{"value":[]}`)
	}))
	defer srv.Close()

	app, _, _ := testApp(srv)
	err := run(app, "mail", "search", "--folder", "Nope")
	if err == nil || err.Error() != "Folder not found: Nope" {
		t.Errorf("err = %v", err)
	}
}

func TestMailSearchNoMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"value":[]}`)
	}))
	defer srv.Close()

	app, out, _ := testApp(srv)
	if err := run(app, "mail", "search"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No messages found.") {
		t.Errorf("got %q", out.String())
	}
}

func TestMailSearchNotConfigured(t *testing.T) {
	app, _, _ := testApp(nil)
	app.NewClient = func() (*graph.Client, error) {
		return nil, errors.New("Not configured. Run: outlook auth login --client-id <ID>")
	}
	err := run(app, "mail", "search")
	if err == nil || err.Error() != "Not configured. Run: outlook auth login --client-id <ID>" {
		t.Errorf("err = %v", err)
	}
}

// ── mail read ──────────────────────────────────────────────────

func TestMailReadNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	app, _, _ := testApp(srv)
	err := run(app, "mail", "read", "missing-id")
	if err == nil || err.Error() != "Message not found: missing-id" {
		t.Errorf("err = %v", err)
	}
}

func TestMailRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/me/messages/msg-1" {
			t.Errorf("path = %q", r.URL.Path)
		}
		fmt.Fprint(w, `{
			"id":"msg-1","subject":"Hello",
			"from":{"emailAddress":{"name":"Alice","address":"alice@example.com"}},
			"toRecipients":[{"emailAddress":{"address":"me@example.com"}}],
			"receivedDateTime":"2026-07-01T12:00:00Z",
			"body":{"contentType":"Text","content":"The body."}
		}`)
	}))
	defer srv.Close()

	app, out, _ := testApp(srv)
	if err := run(app, "mail", "read", "msg-1"); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"Hello", "The body.", "Alice <alice@example.com>"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

// ── mail send ──────────────────────────────────────────────────

func TestMailSend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/me/sendMail" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	app, out, _ := testApp(srv)
	err := run(app, "mail", "send", "--to", "a@b.c", "--subject", "Hi", "--body", "Text")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "OK: Message sent.") {
		t.Errorf("got %q", out.String())
	}
}

func TestMailSendMissingRequiredFlag(t *testing.T) {
	app, _, _ := testApp(nil)
	if err := run(app, "mail", "send", "--to", "a@b.c"); err == nil {
		t.Error("expected error for missing required flags")
	}
}

func TestMailSendWithHTMLFlag(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	app, out, _ := testApp(srv)
	err := run(app, "mail", "send", "--to", "a@b.c", "--subject", "Hi", "--body", "<h1>HTML</h1>", "--html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotBody, `"contentType":"HTML"`) {
		t.Errorf("body missing HTML contentType: %q", gotBody)
	}
	// JSON escapes < and > as \u003c and \u003e
	if !strings.Contains(gotBody, "HTML") {
		t.Errorf("body missing HTML content: %q", gotBody)
	}
	if !strings.Contains(out.String(), "OK: Message sent.") {
		t.Errorf("got %q", out.String())
	}
}

// ── mail draft ─────────────────────────────────────────────────

func TestMailDraftAllFields(t *testing.T) {
	var gotReq *http.Request
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r.Clone(context.Background())
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"draft-1"}`)
	}))
	defer srv.Close()

	app, out, _ := testApp(srv)
	err := run(app, "mail", "draft",
		"--to", "alice@example.com",
		"--to", "bob@example.com",
		"--cc", "carol@example.com",
		"--bcc", "dave@example.com",
		"--subject", "Quarterly report",
		"--body", "Draft body text",
		"--importance", "high")
	if err != nil {
		t.Fatal(err)
	}
	if gotReq.Method != http.MethodPost || gotReq.URL.Path != "/me/messages" {
		t.Errorf("%s %s", gotReq.Method, gotReq.URL.Path)
	}
	for _, want := range []string{
		`"alice@example.com"`, `"bob@example.com"`,
		"ccRecipients", "bccRecipients",
		`"subject":"Quarterly report"`, `"importance":"high"`,
	} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("body %q missing %q", gotBody, want)
		}
	}
	if !strings.Contains(out.String(), "OK: Draft saved to Drafts folder.") {
		t.Errorf("got %q", out.String())
	}
}

func TestMailDraftSubjectOnly(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"draft-1"}`)
	}))
	defer srv.Close()

	app, out, _ := testApp(srv)
	if err := run(app, "mail", "draft", "--subject", "Ideas"); err != nil {
		t.Fatal(err)
	}
	if gotBody != `{"subject":"Ideas"}` {
		t.Errorf("body = %q", gotBody)
	}
	if !strings.Contains(out.String(), "OK: Draft saved to Drafts folder.") {
		t.Errorf("got %q", out.String())
	}
}

func TestMailDraftNoFieldsErrors(t *testing.T) {
	app, _, _ := testApp(nil)
	err := run(app, "mail", "draft")
	if err == nil || err.Error() != "Nothing to save: provide at least one of --to/--cc/--bcc/--subject/--body." {
		t.Errorf("err = %v", err)
	}
}

func TestMailDraftInvalidImportance(t *testing.T) {
	app, _, _ := testApp(nil)
	err := run(app, "mail", "draft", "--subject", "Ideas", "--importance", "urgent")
	if err == nil || err.Error() != "Invalid importance: urgent (expected low, normal, or high)" {
		t.Errorf("err = %v", err)
	}
}

func TestMailDraftWithHTMLFlag(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"draft-1"}`)
	}))
	defer srv.Close()

	app, out, _ := testApp(srv)
	err := run(app, "mail", "draft", "--to", "a@b.c", "--subject", "HTML Draft", "--body", "<p>Content</p>", "--html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotBody, `"contentType":"HTML"`) {
		t.Errorf("body missing HTML contentType: %q", gotBody)
	}
	// JSON escapes < and > as \u003c and \u003e
	if !strings.Contains(gotBody, "Content") {
		t.Errorf("body missing HTML content: %q", gotBody)
	}
	if !strings.Contains(out.String(), "OK: Draft saved to Drafts folder.") {
		t.Errorf("got %q", out.String())
	}
}

// ── mail reply ─────────────────────────────────────────────────

func TestMailReply(t *testing.T) {
	var replyPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, `{"id":"msg-1","subject":"Hi","from":{"emailAddress":{"name":"Alice","address":"alice@example.com"}}}`)
			return
		}
		replyPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	app, out, _ := testApp(srv)
	if err := run(app, "mail", "reply", "msg-1", "--body", "Thanks"); err != nil {
		t.Fatal(err)
	}
	if replyPath != "/me/messages/msg-1/reply" {
		t.Errorf("reply path = %q", replyPath)
	}
	if !strings.Contains(out.String(), "OK: Reply sent to Alice <alice@example.com>.") {
		t.Errorf("got %q", out.String())
	}
}

func TestMailReplyAll(t *testing.T) {
	var replyPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, `{"id":"msg-1","from":{"emailAddress":{"address":"a@b.c"}}}`)
			return
		}
		replyPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	app, out, _ := testApp(srv)
	if err := run(app, "mail", "reply", "msg-1", "--body", "Thanks", "--reply-all"); err != nil {
		t.Fatal(err)
	}
	if replyPath != "/me/messages/msg-1/replyAll" {
		t.Errorf("reply path = %q", replyPath)
	}
	if !strings.Contains(out.String(), "OK: Reply sent to all recipients.") {
		t.Errorf("got %q", out.String())
	}
}

func TestMailReplyWithHTMLFlag(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, `{"id":"msg-1","from":{"emailAddress":{"address":"a@b.c"}}}`)
			return
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	app, out, _ := testApp(srv)
	err := run(app, "mail", "reply", "msg-1", "--body", "<b>HTML reply</b>", "--html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotBody, `"contentType":"HTML"`) {
		t.Errorf("body missing HTML contentType: %q", gotBody)
	}
	// JSON escapes < and > as \u003c and \u003e
	if !strings.Contains(gotBody, "HTML reply") {
		t.Errorf("body missing HTML content: %q", gotBody)
	}
	if !strings.Contains(out.String(), "OK: Reply sent to") {
		t.Errorf("got %q", out.String())
	}
}

// ── mail mark ──────────────────────────────────────────────────

func TestMailMarkDefaultsToRead(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	app, out, _ := testApp(srv)
	if err := run(app, "mail", "mark", "msg-1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotBody, `"isRead":true`) {
		t.Errorf("body = %q", gotBody)
	}
	if !strings.Contains(out.String(), "OK: Message marked as read.") {
		t.Errorf("got %q", out.String())
	}
}

func TestMailMarkUnread(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	app, out, _ := testApp(srv)
	if err := run(app, "mail", "mark", "msg-1", "--unread"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotBody, `"isRead":false`) {
		t.Errorf("body = %q", gotBody)
	}
	if !strings.Contains(out.String(), "OK: Message marked as unread.") {
		t.Errorf("got %q", out.String())
	}
}

func TestMailMarkMutuallyExclusive(t *testing.T) {
	app, _, _ := testApp(nil)
	if err := run(app, "mail", "mark", "msg-1", "--read", "--unread"); err == nil {
		t.Error("expected mutually exclusive flag error")
	}
}

// ── cal list ───────────────────────────────────────────────────

const sampleEventsJSON = `{"value":[{
	"id":"ev-1","subject":"Team Meeting",
	"start":{"dateTime":"2026-07-15T09:00:00.0000000","timeZone":"UTC"},
	"end":{"dateTime":"2026-07-15T10:00:00.0000000","timeZone":"UTC"},
	"location":{"displayName":"Room 1"},
	"isAllDay":false,"recurrence":null
}]}`

func TestCalListDefaultRange(t *testing.T) {
	var gotFilter string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/me/calendar/events" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotFilter = r.URL.Query().Get("$filter")
		fmt.Fprint(w, sampleEventsJSON)
	}))
	defer srv.Close()

	app, out, _ := testApp(srv)
	if err := run(app, "cal", "list"); err != nil {
		t.Fatal(err)
	}
	// Default: today 00:00 local to +7 days (2026-07-15 per fake Now).
	if !strings.Contains(gotFilter, "start/dateTime ge '") || !strings.Contains(gotFilter, "end/dateTime le '") {
		t.Errorf("$filter = %q", gotFilter)
	}
	if !strings.Contains(out.String(), "Team Meeting") {
		t.Errorf("got %q", out.String())
	}
}

func TestCalListNoEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"value":[]}`)
	}))
	defer srv.Close()

	app, out, _ := testApp(srv)
	if err := run(app, "cal", "list"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No events found in the given range.") {
		t.Errorf("got %q", out.String())
	}
}

func TestCalListInvalidDate(t *testing.T) {
	app, _, _ := testApp(nil)
	err := run(app, "cal", "list", "--start", "bogus")
	if err == nil || err.Error() != "Invalid date format: bogus (expected YYYY-MM-DD)" {
		t.Errorf("err = %v", err)
	}
}

func TestCalListFilterFlags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filter := r.URL.Query().Get("$filter")
		for _, want := range []string{
			"contains(subject,'sync')",
			"isAllDay eq true",
			"recurrence ne null",
		} {
			if !strings.Contains(filter, want) {
				t.Errorf("$filter %q missing %q", filter, want)
			}
		}
		fmt.Fprint(w, sampleEventsJSON)
	}))
	defer srv.Close()

	app, _, _ := testApp(srv)
	if err := run(app, "cal", "list", "--subject", "sync", "--all-day", "--recurring"); err != nil {
		t.Fatal(err)
	}
}

// ── cal read ───────────────────────────────────────────────────

func TestCalReadNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	app, _, _ := testApp(srv)
	err := run(app, "cal", "read", "missing")
	if err == nil || err.Error() != "Event not found: missing" {
		t.Errorf("err = %v", err)
	}
}

func TestCalRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"id":"ev-1","subject":"Team Meeting",
			"start":{"dateTime":"2026-07-15T09:00:00.0000000","timeZone":"UTC"},
			"end":{"dateTime":"2026-07-15T10:00:00.0000000","timeZone":"UTC"},
			"body":{"contentType":"Text","content":"Weekly sync"}
		}`)
	}))
	defer srv.Close()

	app, out, _ := testApp(srv)
	if err := run(app, "cal", "read", "ev-1"); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Team Meeting") || !strings.Contains(got, "Weekly sync") {
		t.Errorf("got:\n%s", got)
	}
}

// ── cal create ─────────────────────────────────────────────────

func TestCalCreate(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"new"}`)
	}))
	defer srv.Close()

	app, out, _ := testApp(srv)
	err := run(app, "cal", "create",
		"--subject", "Standup",
		"--start", "2026-07-16 09:00",
		"--end", "2026-07-16 09:15",
		"--location", "Zoom")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotBody, `"subject":"Standup"`) {
		t.Errorf("body = %q", gotBody)
	}
	if !strings.Contains(out.String(), "OK: Event created: Standup") {
		t.Errorf("got %q", out.String())
	}
}

func TestCalCreateInvalidDatetime(t *testing.T) {
	app, _, _ := testApp(nil)
	err := run(app, "cal", "create", "--subject", "X", "--start", "not-a-date", "--end", "2026-07-16 09:15")
	if err == nil || err.Error() != "Invalid datetime format: not-a-date (expected YYYY-MM-DD HH:MM)" {
		t.Errorf("err = %v", err)
	}
}

// ── auth ───────────────────────────────────────────────────────

func TestAuthLoginNoClientID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	app, _, _ := testApp(nil)
	err := run(app, "auth", "login")
	if err == nil || err.Error() != "No client ID found. Run: outlook auth login --client-id <ID>" {
		t.Errorf("err = %v", err)
	}
}

func TestAuthLoginSavesConfigAndAuthenticates(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	app, out, _ := testApp(nil)
	var gotClient, gotTenant string
	app.Authenticate = func(ctx context.Context, clientID, tenantID string, w io.Writer) error {
		gotClient, gotTenant = clientID, tenantID
		return nil
	}

	if err := run(app, "auth", "login", "--client-id", "my-client", "--tenant-id", "my-tenant"); err != nil {
		t.Fatal(err)
	}
	if gotClient != "my-client" || gotTenant != "my-tenant" {
		t.Errorf("authenticated with %q/%q", gotClient, gotTenant)
	}
	if !strings.Contains(out.String(), "OK: Authenticated successfully.") {
		t.Errorf("got %q", out.String())
	}

	data, err := os.ReadFile(filepath.Join(home, ".outlook-cli", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "my-client") {
		t.Errorf("config = %q", data)
	}
}

func TestAuthLoginFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	app, _, errBuf := testApp(nil)
	app.Authenticate = func(ctx context.Context, clientID, tenantID string, w io.Writer) error {
		return errors.New("device flow error")
	}
	err := run(app, "auth", "login", "--client-id", "c")
	if err == nil || err.Error() != "Authentication failed." {
		t.Errorf("err = %v", err)
	}
	if !strings.Contains(errBuf.String(), "device flow error") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestAuthLogoutNoToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	app, out, _ := testApp(nil)
	if err := run(app, "auth", "logout"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No token file found; already logged out.") {
		t.Errorf("got %q", out.String())
	}
}

func TestAuthLogoutRemovesToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".outlook-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	tokenPath := filepath.Join(dir, "token.json")
	if err := os.WriteFile(tokenPath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	app, out, _ := testApp(nil)
	if err := run(app, "auth", "logout"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "OK: Logged out — token removed.") {
		t.Errorf("got %q", out.String())
	}
	if _, err := os.Stat(tokenPath); !os.IsNotExist(err) {
		t.Error("token file still exists")
	}
}

func TestAuthStatus(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	app, out, errBuf := testApp(nil)

	if err := run(app, "auth", "status"); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Client ID: not set") || !strings.Contains(got, "Tenant ID: not set") {
		t.Errorf("got %q", got)
	}
	if !strings.Contains(got, "OK: Authenticated.") {
		t.Errorf("got %q", got)
	}

	app.IsAuthenticated = func() bool { return false }
	out.Reset()
	if err := run(app, "auth", "status"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errBuf.String(), "Error: Not authenticated.") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMailConversationPaging(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/me/messages/seed":
			fmt.Fprint(w, `{"conversationId":"conv'1"}`)
		case "/me/messages":
			if r.URL.Query().Get("$skiptoken") == "next" {
				fmt.Fprint(w, `{"value":[{"id":"old","subject":"Earlier","sentDateTime":"2026-01-01T00:00:00Z"}]}`)
				return
			}
			if r.URL.Query().Get("$filter") != "conversationId eq 'conv''1'" || r.URL.Query().Has("$orderby") {
				t.Errorf("query: %v", r.URL.Query())
			}
			fmt.Fprintf(w, `{"value":[{"id":"new","subject":"Later","sentDateTime":"2026-02-01T00:00:00Z"}],"@odata.nextLink":%q}`, srv.URL+"/me/messages?$skiptoken=next")
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	app, out, _ := testApp(srv)
	if err := run(app, "mail", "conversation", "seed"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "Earlier") || strings.Index(text, "Earlier") > strings.Index(text, "Later") {
		t.Fatalf("output: %s", text)
	}
}

func TestConversationSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/me/messages" || r.URL.Query().Get("$filter") != "isRead eq false and conversationId eq 'conv'" {
			t.Errorf("request: %s", r.URL)
		}
		fmt.Fprint(w, sampleMessagesJSON)
	}))
	defer srv.Close()
	app, _, _ := testApp(srv)
	if err := run(app, "mail", "search", "--conversation-id", "conv", "--all-folders", "--unread"); err != nil {
		t.Fatal(err)
	}
}

func TestConversationInvalidFlags(t *testing.T) {
	for _, args := range [][]string{
		{"mail", "search", "text", "--conversation-id", "conv"},
		{"mail", "search", "--all-folders", "--folder", "Inbox"},
		{"mail", "search", "--conversation-id", ""},
		{"mail", "search", "--limit", "0"},
	} {
		app, _, _ := testApp(nil)
		if err := run(app, args...); err == nil {
			t.Errorf("expected error for %v", args)
		}
	}
}

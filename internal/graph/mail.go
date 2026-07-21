package graph

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	brRe     = regexp.MustCompile(`<br\s*/?>`)
	pOpenRe  = regexp.MustCompile(`<p[^>]*>`)
	pCloseRe = regexp.MustCompile(`</p>`)
	tagRe    = regexp.MustCompile(`<[^>]+>`)
	htmlRe   = regexp.MustCompile(`(?i)<(html|div|p|br|table)\b`)
)

// stripHTML removes HTML tags for plain text display.
func stripHTML(text string) string {
	clean := brRe.ReplaceAllString(text, "\n")
	clean = pOpenRe.ReplaceAllString(clean, "\n")
	clean = pCloseRe.ReplaceAllString(clean, "")
	clean = tagRe.ReplaceAllString(clean, "")
	clean = strings.ReplaceAll(clean, "&nbsp;", " ")
	clean = html.UnescapeString(clean)
	return strings.TrimSpace(clean)
}

// looksLikeHTML reports whether text appears to contain HTML markup.
func looksLikeHTML(text string) bool {
	return htmlRe.MatchString(text)
}

// EmailAddress is a Graph emailAddress object.
type EmailAddress struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// String formats the address as "Name <address>".
func (e EmailAddress) String() string {
	switch {
	case e.Name != "" && e.Address != "":
		return e.Name + " <" + e.Address + ">"
	case e.Name != "":
		return e.Name
	default:
		return e.Address
	}
}

// Recipient is a Graph recipient object.
type Recipient struct {
	EmailAddress EmailAddress `json:"emailAddress"`
}

func (r Recipient) String() string { return r.EmailAddress.String() }

// ItemBody is a Graph itemBody object.
type ItemBody struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

// Message is a Graph message.
type Message struct {
	ID               string       `json:"id"`
	Subject          string       `json:"subject"`
	From             *Recipient   `json:"from"`
	ToRecipients     []Recipient  `json:"toRecipients"`
	CcRecipients     []Recipient  `json:"ccRecipients"`
	ReceivedDateTime time.Time    `json:"receivedDateTime"`
	IsRead           bool         `json:"isRead"`
	Importance       string       `json:"importance"`
	HasAttachments   bool         `json:"hasAttachments"`
	Body             *ItemBody    `json:"body"`
	Attachments      []Attachment `json:"attachments,omitempty"`
}

// Attachment is a Graph attachment object.
type Attachment struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ContentType   string `json:"contentType"`
	Size          int    `json:"size"`
	IsInline      bool   `json:"isInline"`
	ContentBytes  string `json:"contentBytes,omitempty"` // base64 encoded
	ODataType     string `json:"@odata.type,omitempty"`
	ContentID     string `json:"contentId,omitempty"`
	LastModified  string `json:"lastModifiedDateTime,omitempty"`
}

type mailFolder struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

// ResolveFolder maps a folder display name to a folder ID. "Inbox" maps to
// the well-known inbox folder; other names are looked up by display name.
func (c *Client) ResolveFolder(ctx context.Context, name string) (string, error) {
	if name == "Inbox" {
		return "inbox", nil
	}
	v := url.Values{}
	v.Set("$filter", "displayName eq '"+escapeODataString(name)+"'")
	var out listResponse[mailFolder]
	if err := c.do(ctx, http.MethodGet, "/me/mailFolders", v, nil, &out); err != nil {
		return "", err
	}
	if len(out.Value) == 0 {
		return "", ErrNotFound
	}
	return out.Value[0].ID, nil
}

// ListMessages lists messages in the given folder matching the query.
func (c *Client) ListMessages(ctx context.Context, folderID string, q MailQuery) ([]Message, error) {
	var out listResponse[Message]
	path := "/me/mailFolders/" + url.PathEscape(folderID) + "/messages"
	if err := c.do(ctx, http.MethodGet, path, q.Values(), nil, &out); err != nil {
		return nil, err
	}
	return out.Value, nil
}

// GetMessage fetches a single message by ID.
func (c *Client) GetMessage(ctx context.Context, id string) (*Message, error) {
	var msg Message
	path := "/me/messages/" + url.PathEscape(id)
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// SendMail composes and sends a new message.
func (c *Client) SendMail(ctx context.Context, to, cc, subject, body, contentType string) error {
	if contentType == "" {
		contentType = "Text"
	}
	msg := map[string]any{
		"subject":      subject,
		"body":         ItemBody{ContentType: contentType, Content: body},
		"toRecipients": []Recipient{{EmailAddress: EmailAddress{Address: to}}},
	}
	if cc != "" {
		msg["ccRecipients"] = []Recipient{{EmailAddress: EmailAddress{Address: cc}}}
	}
	payload := map[string]any{"message": msg}
	return c.do(ctx, http.MethodPost, "/me/sendMail", nil, payload, nil)
}

// Draft describes a new draft message. All fields are optional.
type Draft struct {
	To, Cc, Bcc []string
	Subject     string
	Body        string
	ContentType string
	Importance  string
}

// DraftMessage is the response when creating a draft.
type DraftMessage struct {
	ID      string `json:"id"`
	Subject string `json:"subject"`
}

func toRecipients(addrs []string) []Recipient {
	rs := make([]Recipient, len(addrs))
	for i, a := range addrs {
		rs[i] = Recipient{EmailAddress: EmailAddress{Address: a}}
	}
	return rs
}

// CreateDraft saves a new message to the Drafts folder without sending it.
// Returns the created draft message with its ID.
func (c *Client) CreateDraft(ctx context.Context, d Draft) (*DraftMessage, error) {
	msg := map[string]any{}
	if d.Subject != "" {
		msg["subject"] = d.Subject
	}
	if d.Body != "" {
		contentType := d.ContentType
		if contentType == "" {
			contentType = "Text"
		}
		msg["body"] = ItemBody{ContentType: contentType, Content: d.Body}
	}
	if len(d.To) > 0 {
		msg["toRecipients"] = toRecipients(d.To)
	}
	if len(d.Cc) > 0 {
		msg["ccRecipients"] = toRecipients(d.Cc)
	}
	if len(d.Bcc) > 0 {
		msg["bccRecipients"] = toRecipients(d.Bcc)
	}
	if d.Importance != "" {
		msg["importance"] = d.Importance
	}
	var out DraftMessage
	if err := c.do(ctx, http.MethodPost, "/me/messages", nil, msg, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SendDraft sends an existing draft message.
func (c *Client) SendDraft(ctx context.Context, msgID string) error {
	path := "/me/messages/" + url.PathEscape(msgID) + "/send"
	return c.do(ctx, http.MethodPost, path, nil, nil, nil)
}

// Reply sends a reply (or reply-all) to a message with the given comment.
// If draft is true, creates a draft reply instead of sending immediately and returns the draft.
// Automatically quotes the original message in the reply body.
func (c *Client) Reply(ctx context.Context, id, comment, contentType string, all, draft bool) (*DraftMessage, error) {
	if contentType == "" {
		contentType = "Text"
	}

	// Fetch the original message to quote it
	originalMsg, err := c.GetMessage(ctx, id)
	if err != nil {
		return nil, err
	}

	// Build the full reply body with quoted original message
	fullBody := buildReplyWithQuote(comment, originalMsg, contentType)

	var action string
	if draft {
		action = "/createReply"
		if all {
			action = "/createReplyAll"
		}
	} else {
		action = "/reply"
		if all {
			action = "/replyAll"
		}
	}
	path := "/me/messages/" + url.PathEscape(id) + action
	payload := map[string]any{
		"message": map[string]any{
			"body": ItemBody{ContentType: contentType, Content: fullBody},
		},
	}

	if draft {
		var out DraftMessage
		if err := c.do(ctx, http.MethodPost, path, nil, payload, &out); err != nil {
			return nil, err
		}
		return &out, nil
	}

	return nil, c.do(ctx, http.MethodPost, path, nil, payload, nil)
}

// SetRead marks a message as read or unread.
func (c *Client) SetRead(ctx context.Context, id string, read bool) error {
	path := "/me/messages/" + url.PathEscape(id)
	payload := map[string]bool{"isRead": read}
	return c.do(ctx, http.MethodPatch, path, nil, payload, nil)
}

// ListAttachments lists all attachments for a message.
func (c *Client) ListAttachments(ctx context.Context, msgID string) ([]Attachment, error) {
	var out listResponse[Attachment]
	path := "/me/messages/" + url.PathEscape(msgID) + "/attachments"
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Value, nil
}

// GetAttachment fetches a single attachment by ID, including content bytes.
func (c *Client) GetAttachment(ctx context.Context, msgID, attID string) (*Attachment, error) {
	var att Attachment
	path := "/me/messages/" + url.PathEscape(msgID) + "/attachments/" + url.PathEscape(attID)
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &att); err != nil {
		return nil, err
	}
	return &att, nil
}

// AddAttachment adds a file attachment to a message (typically a draft).
// For files < 3MB, uses direct upload. Content should be base64-encoded.
func (c *Client) AddAttachment(ctx context.Context, msgID, name, contentType, contentBase64 string) error {
	path := "/me/messages/" + url.PathEscape(msgID) + "/attachments"
	payload := map[string]any{
		"@odata.type":  "#microsoft.graph.fileAttachment",
		"name":         name,
		"contentType":  contentType,
		"contentBytes": contentBase64,
	}
	return c.do(ctx, http.MethodPost, path, nil, payload, nil)
}

// buildReplyWithQuote constructs a reply body that includes the user's comment
// followed by a quoted version of the original message (like Outlook does).
func buildReplyWithQuote(comment string, original *Message, contentType string) string {
	// Extract original message details
	fromAddr := ""
	if original.From != nil {
		fromAddr = original.From.String()
	}

	toAddrs := ""
	for i, r := range original.ToRecipients {
		if i > 0 {
			toAddrs += "; "
		}
		toAddrs += r.String()
	}

	sentTime := original.ReceivedDateTime.Format("Monday, January 2, 2006 3:04 PM")
	subject := original.Subject

	originalBody := ""
	if original.Body != nil {
		originalBody = original.Body.Content
	}

	if contentType == "HTML" {
		// HTML format reply with quoted message
		return comment + "<br><br><hr>" +
			"<div><b>From:</b> " + htmlEscape(fromAddr) + "<br>" +
			"<b>Sent:</b> " + sentTime + "<br>" +
			"<b>To:</b> " + htmlEscape(toAddrs) + "<br>" +
			"<b>Subject:</b> " + htmlEscape(subject) + "<br></div><br>" +
			originalBody
	}

	// Plain text format reply - strip HTML if original was HTML
	if looksLikeHTML(originalBody) {
		originalBody = stripHTML(originalBody)
	}

	// Plain text format reply with quoted message
	return comment + "\n\n________________________________\n" +
		"From: " + fromAddr + "\n" +
		"Sent: " + sentTime + "\n" +
		"To: " + toAddrs + "\n" +
		"Subject: " + subject + "\n\n" +
		originalBody
}

// htmlEscape escapes special HTML characters for safe display
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

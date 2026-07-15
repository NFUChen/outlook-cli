package graph

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

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
	ID               string      `json:"id"`
	Subject          string      `json:"subject"`
	From             *Recipient  `json:"from"`
	ToRecipients     []Recipient `json:"toRecipients"`
	CcRecipients     []Recipient `json:"ccRecipients"`
	ReceivedDateTime time.Time   `json:"receivedDateTime"`
	IsRead           bool        `json:"isRead"`
	Importance       string      `json:"importance"`
	HasAttachments   bool        `json:"hasAttachments"`
	Body             *ItemBody   `json:"body"`
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
func (c *Client) SendMail(ctx context.Context, to, cc, subject, body string) error {
	msg := map[string]any{
		"subject":      subject,
		"body":         ItemBody{ContentType: "Text", Content: body},
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
	Importance  string
}

func toRecipients(addrs []string) []Recipient {
	rs := make([]Recipient, len(addrs))
	for i, a := range addrs {
		rs[i] = Recipient{EmailAddress: EmailAddress{Address: a}}
	}
	return rs
}

// CreateDraft saves a new message to the Drafts folder without sending it.
func (c *Client) CreateDraft(ctx context.Context, d Draft) error {
	msg := map[string]any{}
	if d.Subject != "" {
		msg["subject"] = d.Subject
	}
	if d.Body != "" {
		msg["body"] = ItemBody{ContentType: "Text", Content: d.Body}
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
	return c.do(ctx, http.MethodPost, "/me/messages", nil, msg, nil)
}

// Reply sends a reply (or reply-all) to a message with the given comment.
func (c *Client) Reply(ctx context.Context, id, comment string, all bool) error {
	action := "/reply"
	if all {
		action = "/replyAll"
	}
	path := "/me/messages/" + url.PathEscape(id) + action
	payload := map[string]string{"comment": comment}
	return c.do(ctx, http.MethodPost, path, nil, payload, nil)
}

// SetRead marks a message as read or unread.
func (c *Client) SetRead(ctx context.Context, id string, read bool) error {
	path := "/me/messages/" + url.PathEscape(id)
	payload := map[string]bool{"isRead": read}
	return c.do(ctx, http.MethodPatch, path, nil, payload, nil)
}

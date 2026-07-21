package cmd

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/mhattingpete/outlook-cli/internal/graph"
)

// attachFile represents a file to attach to an email.
type attachFile struct {
	Path        string
	Name        string
	ContentType string
	Content     []byte
}

// readAttachments reads files from paths and prepares them for attachment.
func readAttachments(paths []string) ([]attachFile, error) {
	var files []attachFile
	for _, p := range paths {
		content, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("failed to read attachment %s: %w", p, err)
		}
		name := filepath.Base(p)
		contentType := mime.TypeByExtension(filepath.Ext(p))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		files = append(files, attachFile{
			Path:        p,
			Name:        name,
			ContentType: contentType,
			Content:     content,
		})
	}
	return files, nil
}

// addAttachmentsToMessage adds attachments to a message (draft).
func addAttachmentsToMessage(ctx context.Context, client *graph.Client, msgID string, files []attachFile) error {
	for _, f := range files {
		encoded := base64.StdEncoding.EncodeToString(f.Content)
		if err := client.AddAttachment(ctx, msgID, f.Name, f.ContentType, encoded); err != nil {
			return fmt.Errorf("failed to attach %s: %w", f.Name, err)
		}
	}
	return nil
}

func newMailCmd(app *App) *cobra.Command {
	c := &cobra.Command{
		Use:   "mail",
		Short: "Email commands",
	}
	c.AddCommand(
		newMailSearchCmd(app),
		newMailReadCmd(app),
		newMailSendCmd(app),
		newMailDraftCmd(app),
		newMailReplyCmd(app),
		newMailMarkCmd(app),
		newMailAttachmentCmd(app),
	)
	return c
}

func newMailSearchCmd(app *App) *cobra.Command {
	var (
		folder             string
		limit              int
		sender             string
		startDate, endDate string
		unread             bool
		important          bool
		hasAttachments     bool
	)
	c := &cobra.Command{
		Use:   "search [QUERY]",
		Short: "Search for messages in a mail folder.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var query string
			if len(args) > 0 {
				query = args[0]
			}

			client, err := app.NewClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			folderID, err := client.ResolveFolder(ctx, folder)
			if errors.Is(err, graph.ErrNotFound) {
				return fmt.Errorf("Folder not found: %s", folder)
			}
			if err != nil {
				return err
			}

			hasFilters := sender != "" || startDate != "" || endDate != "" ||
				unread || important || hasAttachments

			q := graph.MailQuery{Limit: limit}
			if query != "" {
				q.Search = query
				if hasFilters {
					app.Printer.Warn("Filters are ignored when using text search. " +
						"Microsoft Graph API does not support combining search with OData filters.")
				}
			} else if hasFilters {
				if startDate != "" {
					t, err := parseDate(startDate)
					if err != nil {
						return err
					}
					q.Start = t
				}
				if endDate != "" {
					t, err := parseDate(endDate)
					if err != nil {
						return err
					}
					q.End = t
				}
				q.Unread = unread
				q.Important = important
				q.HasAttachments = hasAttachments
				q.Sender = sender
			}

			msgs, err := client.ListMessages(ctx, folderID, q)
			if err != nil {
				return err
			}

			if len(msgs) == 0 {
				app.Printer.Println("No messages found.")
				return nil
			}
			app.Printer.MailTable(msgs)
			return nil
		},
	}
	flags := c.Flags()
	flags.StringVar(&folder, "folder", "Inbox", "Folder name to search in")
	flags.IntVar(&limit, "limit", 25, "Maximum number of messages to return")
	flags.StringVar(&sender, "from", "", "Filter by sender email address")
	flags.StringVar(&startDate, "start-date", "", "Messages received after this date (YYYY-MM-DD)")
	flags.StringVar(&endDate, "end-date", "", "Messages received before this date (YYYY-MM-DD)")
	flags.BoolVar(&unread, "unread", false, "Show only unread messages")
	flags.BoolVar(&important, "important", false, "Show only high-importance messages")
	flags.BoolVar(&hasAttachments, "has-attachments", false, "Show only messages with attachments")
	// --sender is an alias for --from.
	flags.SetNormalizeFunc(func(f *pflag.FlagSet, name string) pflag.NormalizedName {
		if name == "sender" {
			name = "from"
		}
		return pflag.NormalizedName(name)
	})
	return c
}

func newMailReadCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "read MESSAGE_ID",
		Short: "Read a single message by ID.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := app.NewClient()
			if err != nil {
				return err
			}
			msg, err := client.GetMessage(cmd.Context(), args[0])
			if errors.Is(err, graph.ErrNotFound) {
				return fmt.Errorf("Message not found: %s", args[0])
			}
			if err != nil {
				return err
			}
			app.Printer.MailDetail(msg)
			return nil
		},
	}
}

func newMailSendCmd(app *App) *cobra.Command {
	var to, subject, body, cc string
	var html bool
	var attachments []string
	c := &cobra.Command{
		Use:   "send",
		Short: "Compose and send a new message.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := app.NewClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			contentType := "Text"
			if html {
				contentType = "HTML"
			}

			// If no attachments, use simple SendMail
			if len(attachments) == 0 {
				if err := client.SendMail(ctx, to, cc, subject, body, contentType); err != nil {
					return err
				}
				app.Printer.Success("Message sent.")
				return nil
			}

			// With attachments: create draft → add attachments → send
			files, err := readAttachments(attachments)
			if err != nil {
				return err
			}

			draft := graph.Draft{
				To:          []string{to},
				Subject:     subject,
				Body:        body,
				ContentType: contentType,
			}
			if cc != "" {
				draft.Cc = []string{cc}
			}

			draftMsg, err := client.CreateDraft(ctx, draft)
			if err != nil {
				return err
			}

			if err := addAttachmentsToMessage(ctx, client, draftMsg.ID, files); err != nil {
				return err
			}

			if err := client.SendDraft(ctx, draftMsg.ID); err != nil {
				return err
			}

			app.Printer.Success(fmt.Sprintf("Message sent with %d attachment(s).", len(files)))
			return nil
		},
	}
	c.Flags().StringVar(&to, "to", "", "Recipient email address")
	c.Flags().StringVar(&subject, "subject", "", "Message subject")
	c.Flags().StringVar(&body, "body", "", "Message body text")
	c.Flags().StringVar(&cc, "cc", "", "CC email address")
	c.Flags().BoolVar(&html, "html", false, "Send body as HTML instead of plain text")
	c.Flags().StringArrayVar(&attachments, "attach", nil, "File to attach (repeatable)")
	_ = c.MarkFlagRequired("to")
	_ = c.MarkFlagRequired("subject")
	_ = c.MarkFlagRequired("body")
	return c
}

func newMailDraftCmd(app *App) *cobra.Command {
	var to, cc, bcc []string
	var subject, body, importance string
	var html bool
	var attachments []string
	c := &cobra.Command{
		Use:   "draft",
		Short: "Compose a new message and save it as a draft (does not send).",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(to) == 0 && len(cc) == 0 && len(bcc) == 0 && subject == "" && body == "" && len(attachments) == 0 {
				return errors.New("Nothing to save: provide at least one of --to/--cc/--bcc/--subject/--body/--attach.")
			}
			if importance != "" {
				switch strings.ToLower(importance) {
				case "low", "normal", "high":
					importance = strings.ToLower(importance)
				default:
					return fmt.Errorf("Invalid importance: %s (expected low, normal, or high)", importance)
				}
			}

			client, err := app.NewClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			contentType := "Text"
			if html {
				contentType = "HTML"
			}
			draft := graph.Draft{
				To: to, Cc: cc, Bcc: bcc,
				Subject: subject, Body: body, ContentType: contentType, Importance: importance,
			}
			draftMsg, err := client.CreateDraft(ctx, draft)
			if err != nil {
				return err
			}

			// Add attachments if any
			if len(attachments) > 0 {
				files, err := readAttachments(attachments)
				if err != nil {
					return err
				}
				if err := addAttachmentsToMessage(ctx, client, draftMsg.ID, files); err != nil {
					return err
				}
				app.Printer.Success(fmt.Sprintf("Draft saved with %d attachment(s).", len(files)))
			} else {
				app.Printer.Success("Draft saved to Drafts folder.")
			}
			return nil
		},
	}
	flags := c.Flags()
	flags.StringArrayVar(&to, "to", nil, "Recipient email address (repeatable)")
	flags.StringArrayVar(&cc, "cc", nil, "CC email address (repeatable)")
	flags.StringArrayVar(&bcc, "bcc", nil, "BCC email address (repeatable)")
	flags.StringVar(&subject, "subject", "", "Message subject")
	flags.StringVar(&body, "body", "", "Message body text")
	flags.StringVar(&importance, "importance", "", "Importance: low, normal, or high")
	flags.BoolVar(&html, "html", false, "Format body as HTML instead of plain text")
	flags.StringArrayVar(&attachments, "attach", nil, "File to attach (repeatable)")
	return c
}

func newMailReplyCmd(app *App) *cobra.Command {
	var body string
	var replyAll bool
	var html bool
	var draft bool
	var attachments []string
	c := &cobra.Command{
		Use:   "reply MESSAGE_ID",
		Short: "Reply to a message by ID.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := app.NewClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			msg, err := client.GetMessage(ctx, args[0])
			if errors.Is(err, graph.ErrNotFound) {
				return fmt.Errorf("Message not found: %s", args[0])
			}
			if err != nil {
				return err
			}

			contentType := "Text"
			if html {
				contentType = "HTML"
			}

			// If attachments, must create draft first, add attachments, then optionally send
			if len(attachments) > 0 {
				files, err := readAttachments(attachments)
				if err != nil {
					return err
				}

				// Create reply draft
				draftMsg, err := client.Reply(ctx, args[0], body, contentType, replyAll, true)
				if err != nil {
					return err
				}

				// Add attachments
				if err := addAttachmentsToMessage(ctx, client, draftMsg.ID, files); err != nil {
					return err
				}

				if draft {
					app.Printer.Success(fmt.Sprintf("Reply draft saved with %d attachment(s).", len(files)))
				} else {
					// Send the draft
					if err := client.SendDraft(ctx, draftMsg.ID); err != nil {
						return err
					}
					target := "all recipients"
					if !replyAll && msg.From != nil {
						target = msg.From.String()
					}
					app.Printer.Success(fmt.Sprintf("Reply sent to %s with %d attachment(s).", target, len(files)))
				}
				return nil
			}

			// No attachments - use standard reply flow
			if _, err := client.Reply(ctx, args[0], body, contentType, replyAll, draft); err != nil {
				return err
			}

			if draft {
				app.Printer.Success("Reply draft saved to Drafts folder.")
			} else {
				target := "all recipients"
				if !replyAll && msg.From != nil {
					target = msg.From.String()
				}
				app.Printer.Success(fmt.Sprintf("Reply sent to %s.", target))
			}
			return nil
		},
	}
	c.Flags().StringVar(&body, "body", "", "Reply body text")
	c.Flags().BoolVar(&replyAll, "reply-all", false, "Reply to all recipients")
	c.Flags().BoolVar(&html, "html", false, "Format reply body as HTML instead of plain text")
	c.Flags().BoolVar(&draft, "draft", false, "Save as draft instead of sending immediately")
	c.Flags().StringArrayVar(&attachments, "attach", nil, "File to attach (repeatable)")
	_ = c.MarkFlagRequired("body")
	return c
}

func newMailMarkCmd(app *App) *cobra.Command {
	var markRead, markUnread bool
	c := &cobra.Command{
		Use:   "mark MESSAGE_ID",
		Short: "Mark a message as read or unread.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := app.NewClient()
			if err != nil {
				return err
			}
			read := !markUnread

			err = client.SetRead(cmd.Context(), args[0], read)
			if errors.Is(err, graph.ErrNotFound) {
				return fmt.Errorf("Message not found: %s", args[0])
			}
			if err != nil {
				return err
			}

			if read {
				app.Printer.Success("Message marked as read.")
			} else {
				app.Printer.Success("Message marked as unread.")
			}
			return nil
		},
	}
	c.Flags().BoolVar(&markRead, "read", false, "Mark as read (default)")
	c.Flags().BoolVar(&markUnread, "unread", false, "Mark as unread")
	c.MarkFlagsMutuallyExclusive("read", "unread")
	return c
}

func newMailAttachmentCmd(app *App) *cobra.Command {
	c := &cobra.Command{
		Use:   "attachment",
		Short: "Attachment commands",
	}
	c.AddCommand(
		newMailAttachmentListCmd(app),
		newMailAttachmentDownloadCmd(app),
	)
	return c
}

func newMailAttachmentListCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "list MESSAGE_ID",
		Short: "List attachments for a message.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := app.NewClient()
			if err != nil {
				return err
			}
			attachments, err := client.ListAttachments(cmd.Context(), args[0])
			if errors.Is(err, graph.ErrNotFound) {
				return fmt.Errorf("Message not found: %s", args[0])
			}
			if err != nil {
				return err
			}
			if len(attachments) == 0 {
				app.Printer.Println("No attachments found.")
				return nil
			}
			app.Printer.AttachmentTable(attachments)
			return nil
		},
	}
}

func newMailAttachmentDownloadCmd(app *App) *cobra.Command {
	var output string
	c := &cobra.Command{
		Use:   "download MESSAGE_ID ATTACHMENT_ID",
		Short: "Download an attachment.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := app.NewClient()
			if err != nil {
				return err
			}
			att, err := client.GetAttachment(cmd.Context(), args[0], args[1])
			if errors.Is(err, graph.ErrNotFound) {
				return fmt.Errorf("Attachment not found: %s", args[1])
			}
			if err != nil {
				return err
			}

			// Decode base64 content
			content, err := base64.StdEncoding.DecodeString(att.ContentBytes)
			if err != nil {
				return fmt.Errorf("failed to decode attachment: %w", err)
			}

			// Determine output filename
			filename := output
			if filename == "" {
				filename = att.Name
			}

			if err := os.WriteFile(filename, content, 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}

			app.Printer.Success(fmt.Sprintf("Downloaded: %s (%s)", filename, formatSize(len(content))))
			return nil
		},
	}
	c.Flags().StringVarP(&output, "output", "o", "", "Output filename (defaults to attachment name)")
	return c
}

// formatSize formats bytes into human-readable size.
func formatSize(bytes int) string {
	const (
		KB = 1024
		MB = KB * 1024
	)
	switch {
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

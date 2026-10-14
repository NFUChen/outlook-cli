package cmd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mhattingpete/outlook-cli/internal/graph"
)

// maxConversationMessages caps how many messages a conversation lookup fetches.
const maxConversationMessages = 500

func validateMailScope(limit int, allFolders, folderSet bool) error {
	if limit <= 0 {
		return errors.New("--limit must be greater than 0")
	}
	if allFolders && folderSet {
		return errors.New("--all-folders cannot be combined with --folder")
	}
	return nil
}

// resolveMailScope returns the folder ID to query, or "" for the whole mailbox.
func resolveMailScope(ctx context.Context, client *graph.Client, folder string, allFolders bool) (string, error) {
	if allFolders {
		return "", nil
	}
	folderID, err := client.ResolveFolder(ctx, folder)
	if errors.Is(err, graph.ErrNotFound) {
		return "", fmt.Errorf("Folder not found: %s", folder)
	}
	return folderID, err
}

func messageTime(m graph.Message) time.Time {
	if !m.SentDateTime.IsZero() {
		return m.SentDateTime
	}
	return m.ReceivedDateTime
}

// sortMessagesByTime sorts by sent time (falling back to received time).
func sortMessagesByTime(msgs []graph.Message, ascending bool) {
	sort.SliceStable(msgs, func(i, j int) bool {
		ti, tj := messageTime(msgs[i]), messageTime(msgs[j])
		if ascending {
			return ti.Before(tj)
		}
		return ti.After(tj)
	})
}

func warnIncompleteConversation(app *App) {
	app.Printer.Warn(fmt.Sprintf("Conversation has more than %d messages; results are incomplete.", maxConversationMessages))
}

func newMailConversationCmd(app *App) *cobra.Command {
	var folder string
	c := &cobra.Command{
		Use:   "conversation MESSAGE_ID",
		Short: "List all messages in the same conversation as a message.",
		Long: "List all messages in the same conversation as MESSAGE_ID, oldest first.\n" +
			"By default the whole mailbox is searched (including Sent Items and Deleted Items).",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := app.NewClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			seed, err := client.GetMessage(ctx, args[0])
			if errors.Is(err, graph.ErrNotFound) {
				return fmt.Errorf("Message not found: %s", args[0])
			}
			if err != nil {
				return err
			}
			if seed.ConversationID == "" {
				return fmt.Errorf("Message has no conversation ID: %s", args[0])
			}

			folderID := ""
			if cmd.Flags().Changed("folder") {
				if folderID, err = resolveMailScope(ctx, client, folder, false); err != nil {
					return err
				}
			}

			msgs, complete, err := client.ListAllMessages(ctx, folderID,
				graph.MailQuery{ConversationID: seed.ConversationID}, maxConversationMessages)
			if err != nil {
				return err
			}
			if !complete {
				warnIncompleteConversation(app)
			}
			if len(msgs) == 0 {
				app.Printer.Println("No messages found.")
				return nil
			}
			sortMessagesByTime(msgs, true)
			app.Printer.MailTable(msgs)
			return nil
		},
	}
	c.Flags().StringVar(&folder, "folder", "", "Only list conversation messages in this folder")
	return c
}

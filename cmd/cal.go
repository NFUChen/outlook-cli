package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mhattingpete/outlook-cli/internal/graph"
)

func newCalCmd(app *App) *cobra.Command {
	c := &cobra.Command{
		Use:   "cal",
		Short: "Calendar commands",
	}
	c.AddCommand(newCalListCmd(app), newCalReadCmd(app), newCalCreateCmd(app))
	return c
}

func newCalListCmd(app *App) *cobra.Command {
	var (
		start, end          string
		limit               int
		subject             string
		location, organizer string
		allDay, recurring   bool
	)
	c := &cobra.Command{
		Use:   "list",
		Short: "List calendar events in a date range.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var startDt time.Time
			if start != "" {
				t, err := parseDate(start)
				if err != nil {
					return err
				}
				startDt = t
			} else {
				now := app.Now()
				startDt = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			}

			var endDt time.Time
			if end != "" {
				t, err := parseDate(end)
				if err != nil {
					return err
				}
				endDt = t
			} else {
				endDt = startDt.Add(7 * 24 * time.Hour)
			}

			client, err := app.NewClient()
			if err != nil {
				return err
			}

			q := graph.EventQuery{
				Start:     startDt,
				End:       endDt,
				Subject:   subject,
				Location:  location,
				Organizer: organizer,
				AllDay:    allDay,
				Recurring: recurring,
				Limit:     limit,
			}
			events, err := client.ListEvents(cmd.Context(), q)
			if err != nil {
				return err
			}

			if len(events) == 0 {
				app.Printer.Println("No events found in the given range.")
				return nil
			}
			app.Printer.EventTable(events)
			return nil
		},
	}
	flags := c.Flags()
	flags.StringVar(&start, "start", "", "Start date (YYYY-MM-DD)")
	flags.StringVar(&end, "end", "", "End date (YYYY-MM-DD)")
	flags.IntVar(&limit, "limit", 25, "Max events to return")
	flags.StringVar(&subject, "subject", "", "Filter by subject keyword")
	flags.StringVar(&location, "location", "", "Filter by location keyword")
	flags.StringVar(&organizer, "organizer", "", "Filter by organizer email")
	flags.BoolVar(&allDay, "all-day", false, "Show only all-day events")
	flags.BoolVar(&recurring, "recurring", false, "Show only recurring events")
	return c
}

func newCalReadCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "read EVENT_ID",
		Short: "Read a single calendar event.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := app.NewClient()
			if err != nil {
				return err
			}
			ev, err := client.GetEvent(cmd.Context(), args[0])
			if errors.Is(err, graph.ErrNotFound) {
				return fmt.Errorf("Event not found: %s", args[0])
			}
			if err != nil {
				return err
			}
			app.Printer.EventDetail(ev)
			return nil
		},
	}
}

func newCalCreateCmd(app *App) *cobra.Command {
	var subject, start, end, body, location string
	c := &cobra.Command{
		Use:   "create",
		Short: "Create a new calendar event.",
		RunE: func(cmd *cobra.Command, args []string) error {
			startDt, err := parseDateTime(start)
			if err != nil {
				return err
			}
			endDt, err := parseDateTime(end)
			if err != nil {
				return err
			}

			client, err := app.NewClient()
			if err != nil {
				return err
			}

			ev := graph.NewEvent{
				Subject:  subject,
				Start:    startDt,
				End:      endDt,
				Body:     body,
				Location: location,
			}
			if err := client.CreateEvent(cmd.Context(), ev); err != nil {
				return err
			}
			app.Printer.Success(fmt.Sprintf("Event created: %s", subject))
			return nil
		},
	}
	c.Flags().StringVar(&subject, "subject", "", "Event subject")
	c.Flags().StringVar(&start, "start", "", "Start datetime (YYYY-MM-DD HH:MM)")
	c.Flags().StringVar(&end, "end", "", "End datetime (YYYY-MM-DD HH:MM)")
	c.Flags().StringVar(&body, "body", "", "Event description")
	c.Flags().StringVar(&location, "location", "", "Event location")
	_ = c.MarkFlagRequired("subject")
	_ = c.MarkFlagRequired("start")
	_ = c.MarkFlagRequired("end")
	return c
}

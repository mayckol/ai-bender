package main

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mayckol/ai-bender/internal/session"
)

const (
	ExitSessionNotFound = 50
	ExitSessionInvalid  = 51
)

func newSessionsCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Inspect on-disk sessions written by Claude during slash-command runs",
	}
	cmd.AddCommand(newSessionsListCmd(g))
	cmd.AddCommand(newSessionsShowCmd(g))
	cmd.AddCommand(newSessionsExportCmd(g))
	cmd.AddCommand(newSessionsValidateCmd(g))
	return cmd
}

func newSessionsListCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List past sessions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveProjectRoot(g)
			if err != nil {
				return err
			}
			listings, err := session.List(root)
			if err != nil {
				return err
			}
			return renderSessions(cmd.OutOrStdout(), listings)
		},
	}
}

func newSessionsShowCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show <session-id>",
		Short: "Print every event from events.jsonl in original order",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveProjectRoot(g)
			if err != nil {
				return err
			}
			dir, err := session.ResolveSessionDir(root, args[0])
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), err)
				os.Exit(ExitSessionNotFound)
			}
			return session.CopyEvents(dir, cmd.OutOrStdout())
		},
	}
}

func newSessionsExportCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "export <session-id>",
		Short: "Produce a single JSON document with state + events for ingestion by a UI server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveProjectRoot(g)
			if err != nil {
				return err
			}
			dir, err := session.ResolveSessionDir(root, args[0])
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), err)
				os.Exit(ExitSessionNotFound)
			}
			return session.Export(dir, cmd.OutOrStdout())
		},
	}
}

func newSessionsValidateCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "validate <session-id>",
		Short: "Check state.json + events.jsonl against the v1 schema contract",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveProjectRoot(g)
			if err != nil {
				return err
			}
			dir, err := session.ResolveSessionDir(root, args[0])
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), err)
				os.Exit(ExitSessionNotFound)
			}
			violations, err := session.Validate(dir)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(violations) == 0 {
				fmt.Fprintf(out, "ok: %s is schema-compliant\n", args[0])
				return nil
			}
			fmt.Fprintf(out, "%d violation(s) in session %s:\n", len(violations), args[0])
			for _, v := range violations {
				fmt.Fprintf(out, "  %s\n", v)
			}
			os.Exit(ExitSessionInvalid)
			return nil
		},
	}
}

func renderSessions(out io.Writer, listings []session.Listing) error {
	if len(listings) == 0 {
		fmt.Fprintln(out, "no sessions found (run a slash command in Claude Code to create one)")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCOMMAND\tSTARTED_AT\tDURATION\tSTATUS\tFILES\tFINDINGS")
	for _, l := range listings {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
			l.ID, l.State.Command, l.State.StartedAt.Format("2006-01-02T15:04:05Z"),
			l.Duration, l.State.Status, l.State.FilesChanged, l.State.FindingsCount)
	}
	return w.Flush()
}

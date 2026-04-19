package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mayckol/ai-bender/internal/session"
)

const (
	ExitSessionNotFound = 50
	ExitSessionInvalid  = 51
	ExitClearNeedsTarget = 73
)

func newSessionsCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Inspect and manage on-disk sessions written by Claude during slash-command runs",
	}
	cmd.AddCommand(newSessionsListCmd(g))
	cmd.AddCommand(newSessionsShowCmd(g))
	cmd.AddCommand(newSessionsExportCmd(g))
	cmd.AddCommand(newSessionsValidateCmd(g))
	cmd.AddCommand(newSessionsClearCmd(g))
	cmd.AddCommand(newSessionPRCmd(g))
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

func newSessionsClearCmd(g *globalFlags) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "clear [<session-id>]",
		Short: "Remove a session (or every session with --all) and its scout cache; artifacts are preserved",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveProjectRoot(g)
			if err != nil {
				return err
			}
			switch {
			case all && len(args) > 0:
				fmt.Fprintln(cmd.ErrOrStderr(), "sessions clear: pass either --all or <session-id>, not both")
				os.Exit(ExitClearNeedsTarget)
			case all:
				removed, err := session.ClearAll(root)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "cleared %d session(s) under %s\n", removed, session.SessionsRoot(root))
				return nil
			case len(args) == 0:
				fmt.Fprintln(cmd.ErrOrStderr(), "sessions clear: specify <session-id> or pass --all")
				os.Exit(ExitClearNeedsTarget)
			}
			id := args[0]
			if err := session.Clear(root, id); err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					fmt.Fprintf(cmd.ErrOrStderr(), "sessions clear: %s not found\n", id)
					os.Exit(ExitSessionNotFound)
				}
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "cleared %s\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "remove every session directory and the whole scout cache (artifacts preserved)")
	return cmd
}

func renderSessions(out io.Writer, listings []session.Listing) error {
	if len(listings) == 0 {
		fmt.Fprintln(out, "no sessions found (run a slash command in Claude Code to create one)")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCOMMAND\tSTARTED_AT\tDURATION\tSTATUS\tFILES\tFINDINGS\tWORKTREE\tBRANCH\tPR")
	for _, l := range listings {
		worktreeStatus := "legacy"
		branch := "—"
		prURL := "—"
		if !l.State.IsLegacy() {
			worktreeStatus = string(l.State.Worktree.Status)
			if l.State.SessionBranch != "" {
				branch = trimHeadsPrefix(l.State.SessionBranch)
			}
		}
		if l.State.PullRequest != nil {
			prURL = l.State.PullRequest.URL
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%d\t%s\t%s\t%s\n",
			l.ID, l.State.Command, l.State.StartedAt.Format("2006-01-02T15:04:05Z"),
			l.Duration, l.State.Status, l.State.FilesChanged, l.State.FindingsCount,
			worktreeStatus, branch, prURL)
	}
	return w.Flush()
}

func trimHeadsPrefix(ref string) string {
	const p = "refs/heads/"
	if len(ref) > len(p) && ref[:len(p)] == p {
		return ref[len(p):]
	}
	return ref
}

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mayckol/ai-bender/internal/worktree"
)

func newWorktreeCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worktree",
		Short: "Manage per-session git worktrees created for isolated pipeline runs",
		Long: `bender worktree manages the per-session git worktrees that isolate each pipeline
run from the main working tree. Every bender pipeline session lives inside its own
worktree on its own branch (bender/session/<id>) — this subcommand tree creates,
lists, removes, and bulk-prunes them.`,
	}
	cmd.AddCommand(newWorktreeCreateCmd(g))
	cmd.AddCommand(newWorktreeListCmd(g))
	cmd.AddCommand(newWorktreeRemoveCmd(g))
	cmd.AddCommand(newWorktreePruneCmd(g))
	return cmd
}

func newWorktreeCreateCmd(g *globalFlags) *cobra.Command {
	var baseBranch, workflowID, workflowParent string
	cmd := &cobra.Command{
		Use:   "create <session-id>",
		Short: "Create an isolated worktree for a pipeline session",
		Long: `Create a dedicated git worktree for a new pipeline session. Orchestrator
skills call this at session start so every stage write lands inside the worktree
rather than the main working tree. The resulting directory and branch are printed
to stdout for downstream scripting.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveProjectRoot(g)
			if err != nil {
				return err
			}
			out, err := worktree.Create(context.Background(), worktree.CreateInput{
				RepoRoot:                root,
				SessionID:               args[0],
				BaseBranch:              baseBranch,
				Command:                 "bender worktree create",
				Runner:                  &worktree.ExecRunner{},
				WorkflowID:              workflowID,
				WorkflowParentSessionID: workflowParent,
			})
			if err != nil {
				return mapWorktreeError(cmd.ErrOrStderr(), err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "worktree: %s\nbranch:   %s\n",
				out.WorktreePath, out.Branch)
			return nil
		},
	}
	cmd.Flags().StringVar(&baseBranch, "base-branch", "", "base ref to fork the session branch from (defaults to current branch)")
	cmd.Flags().StringVar(&workflowID, "workflow-id", "", "persist this workflow id in state.json (links TDD→/ghu sessions into one live view)")
	cmd.Flags().StringVar(&workflowParent, "workflow-parent", "", "parent session id when inheriting a workflow id")
	return cmd
}

func newWorktreeListCmd(g *globalFlags) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List every session's worktree status, path, and branch",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveProjectRoot(g)
			if err != nil {
				return err
			}
			rows, err := worktree.List(root)
			if err != nil {
				return err
			}
			if asJSON {
				return renderWorktreeJSON(cmd.OutOrStdout(), rows)
			}
			return renderWorktreeTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit one JSON object per line")
	return cmd
}

func newWorktreeRemoveCmd(g *globalFlags) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "remove <session-id>",
		Short: "Remove a session's worktree (directory + git registration)",
		Long: `Remove a single session's worktree while preserving the session branch and the
session's event/state history. If the directory was deleted out-of-band, reconcile
the git worktree metadata instead. Refuses on active sessions (exit 21).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveProjectRoot(g)
			if err != nil {
				return err
			}
			res, err := worktree.Remove(context.Background(), worktree.RemoveInput{
				RepoRoot:  root,
				SessionID: args[0],
				Force:     force,
				Runner:    &worktree.ExecRunner{},
			})
			if err != nil {
				return mapWorktreeError(cmd.ErrOrStderr(), err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed: %s (reason=%s)\n", args[0], res.Reason)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "force remove even with uncommitted changes in the worktree")
	return cmd
}

func newWorktreePruneCmd(g *globalFlags) *cobra.Command {
	var olderThan string
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Bulk-remove completed or aborted session worktrees",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveProjectRoot(g)
			if err != nil {
				return err
			}
			var age time.Duration
			if olderThan != "" {
				d, err := time.ParseDuration(olderThan)
				if err != nil {
					return fmt.Errorf("prune: parse --older-than: %w", err)
				}
				age = d
			}
			summary, err := worktree.Prune(context.Background(), worktree.PruneInput{
				RepoRoot:  root,
				OlderThan: age,
				Runner:    &worktree.ExecRunner{},
			})
			if err != nil {
				return mapWorktreeError(cmd.ErrOrStderr(), err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d removed, %d reconciled, %d skipped",
				summary.Removed, summary.Reconciled, summary.Skipped)
			if len(summary.FailedSessions) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), ", %d failed: %s",
					len(summary.FailedSessions), strings.Join(summary.FailedSessions, ","))
			}
			fmt.Fprintln(cmd.OutOrStdout())
			return nil
		},
	}
	cmd.Flags().StringVar(&olderThan, "older-than", "", "only prune sessions whose completed_at is older than DURATION (e.g. 72h, 14d not supported — use hours)")
	return cmd
}

func renderWorktreeTable(out io.Writer, rows []worktree.Row) error {
	if len(rows) == 0 {
		fmt.Fprintln(out, "no sessions found")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SESSION\tSTATUS\tPRESENT\tBRANCH\tPATH")
	for _, r := range rows {
		status := "unknown"
		path := r.WorktreePath
		if r.State != nil {
			if r.State.IsLegacy() {
				status = "legacy"
			} else {
				status = string(r.State.Worktree.Status)
			}
		}
		branch := r.Branch
		if branch == "" {
			branch = "—"
		}
		if path == "" {
			path = "—"
		}
		fmt.Fprintf(w, "%s\t%s\t%v\t%s\t%s\n", r.SessionID, status, r.PresentOnDisk, branch, path)
	}
	return w.Flush()
}

type worktreeJSONRow struct {
	SessionID    string `json:"session_id"`
	Status       string `json:"status,omitempty"`
	PresentOnDisk bool  `json:"present_on_disk"`
	Branch       string `json:"branch,omitempty"`
	Path         string `json:"path,omitempty"`
	Legacy       bool   `json:"legacy,omitempty"`
}

func renderWorktreeJSON(out io.Writer, rows []worktree.Row) error {
	enc := json.NewEncoder(out)
	for _, r := range rows {
		row := worktreeJSONRow{
			SessionID:    r.SessionID,
			PresentOnDisk: r.PresentOnDisk,
			Branch:       r.Branch,
			Path:         r.WorktreePath,
		}
		if r.State != nil {
			row.Status = string(r.State.Worktree.Status)
			row.Legacy = r.State.IsLegacy()
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}

// mapWorktreeError writes the error to stderr and exits with the CLI contract
// code for recognised sentinels. Returns the original error unchanged when
// called under a test that expects RunE to surface it (exit never returns).
func mapWorktreeError(stderr io.Writer, err error) error {
	fmt.Fprintln(stderr, err)
	switch {
	case errors.Is(err, worktree.ErrGitUnavailable):
		os.Exit(worktree.ExitGitUnavailable)
	case errors.Is(err, worktree.ErrRepoIncompatible):
		os.Exit(worktree.ExitRepoIncompatible)
	case errors.Is(err, worktree.ErrPlacementRefused):
		os.Exit(worktree.ExitPlacementRefused)
	case errors.Is(err, worktree.ErrBranchCollision):
		os.Exit(worktree.ExitBranchCollision)
	case errors.Is(err, worktree.ErrSessionNotFound):
		os.Exit(worktree.ExitSessionMissing)
	case errors.Is(err, worktree.ErrActiveSession):
		os.Exit(worktree.ExitRefusedActive)
	}
	return err
}

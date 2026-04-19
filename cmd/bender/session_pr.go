package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mayckol/ai-bender/internal/event"
	"github.com/mayckol/ai-bender/internal/pr"
	"github.com/mayckol/ai-bender/internal/session"
	"github.com/mayckol/ai-bender/internal/worktree"
)

// prAdapterFactory is swapped by tests so the command doesn't shell out to
// a real platform CLI.
var prAdapterFactory = func() []pr.Adapter {
	return pr.DefaultAdapters(pr.SystemExecRunner{})
}

func newSessionPRCmd(g *globalFlags) *cobra.Command {
	var (
		draft        bool
		refuseUpdate bool
		asJSON       bool
	)
	cmd := &cobra.Command{
		Use:   "pr <session-id>",
		Short: "Push the session branch and open (or update) a pull request",
		Long: `bender session pr pushes the session branch to the configured remote and
opens a pull request against the session's recorded base branch using the user's
locally installed platform CLI (gh for GitHub remotes today). Strictly opt-in —
no push, no PR, no remote mutation happens until this command runs.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveProjectRoot(g)
			if err != nil {
				return err
			}
			sessionID := args[0]
			sessionDir := filepath.Join(root, ".bender", "sessions", sessionID)
			st, err := session.LoadState(sessionDir)
			if err != nil {
				if errors.Is(err, session.ErrNoState) {
					fmt.Fprintf(cmd.ErrOrStderr(), "pr: session not found: %s\n", sessionID)
					os.Exit(worktree.ExitPRIneligible)
				}
				return err
			}
			if st.IsLegacy() {
				fmt.Fprintln(cmd.ErrOrStderr(), "pr: legacy v1 session has no worktree branch")
				os.Exit(worktree.ExitPRIneligible)
			}
			if st.Status == "running" {
				fmt.Fprintln(cmd.ErrOrStderr(), "pr: session is still active")
				os.Exit(worktree.ExitPRIneligible)
			}
			if err := sessionBranchHasCommits(root, st); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "pr: %v\n", err)
				os.Exit(worktree.ExitPRIneligible)
			}

			remoteURL, remoteName, err := resolveRemote(root)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "pr: %v\n", err)
				os.Exit(worktree.ExitPRNoAdapter)
			}
			adapter, err := pr.SelectAdapter(prAdapterFactory(), remoteURL)
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), err)
				os.Exit(worktree.ExitPRNoAdapter)
			}

			ctx := context.Background()
			if err := adapter.AuthCheck(ctx); err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), err)
				os.Exit(worktree.ExitPRAdapterFailed)
			}

			shortBranch := strings.TrimPrefix(st.SessionBranch, "refs/heads/")
			if err := adapter.Push(ctx, pr.PushInput{
				RepoRoot:     root,
				Remote:       remoteName,
				LocalBranch:  st.SessionBranch,
				RemoteBranch: shortBranch,
			}); err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), err)
				os.Exit(worktree.ExitPRAdapterFailed)
			}

			title, body := derivePRBody(sessionDir, st)
			ref, err := adapter.OpenOrUpdate(ctx, pr.OpenArgs{
				RepoRoot:     root,
				Remote:       remoteName,
				Base:         st.BaseBranch,
				Head:         shortBranch,
				Title:        title,
				Body:         body,
				Draft:        draft,
				RefuseUpdate: refuseUpdate,
			})
			if err != nil {
				if errors.Is(err, pr.ErrExistingPRPresent) {
					// Persist the refusal event even though we refused the op.
					_ = worktree.AppendEvent(sessionDir, &event.Event{
						SchemaVersion: event.SchemaVersion,
						SessionID:     sessionID,
						Timestamp:     time.Now().UTC(),
						Actor:         event.Actor{Kind: event.ActorOrchestrator, Name: "bender-session-pr"},
						Type:          event.TypePRUpdateRefused,
						Payload:       map[string]any{"existing_pr_url": ref.URL},
					})
					fmt.Fprintln(cmd.ErrOrStderr(), err)
					os.Exit(worktree.ExitPRUpdateRefused)
				}
				fmt.Fprintln(cmd.ErrOrStderr(), err)
				os.Exit(worktree.ExitPRAdapterFailed)
			}

			now := time.Now().UTC()
			if st.PullRequest == nil {
				st.PullRequest = &session.PullRequest{
					Remote:         remoteName,
					RemoteURL:      remoteURL,
					BranchOnRemote: shortBranch,
					URL:            ref.URL,
					OpenedAt:       now,
					Adapter:        adapter.Name(),
				}
			} else {
				st.PullRequest.URL = ref.URL // defensive; URL should be stable
			}
			st.PullRequest.LastInvokedAt = now
			if err := session.SaveState(sessionDir, st); err != nil {
				return err
			}
			_ = worktree.AppendEvent(sessionDir, &event.Event{
				SchemaVersion: event.SchemaVersion,
				SessionID:     sessionID,
				Timestamp:     now,
				Actor:         event.Actor{Kind: event.ActorOrchestrator, Name: "bender-session-pr"},
				Type:          event.TypePROpened,
				Payload: map[string]any{
					"remote":           remoteName,
					"branch_on_remote": shortBranch,
					"pr_url":           ref.URL,
					"adapter":          adapter.Name(),
					"opened_at":        now.Format(time.RFC3339),
					"updated":          ref.Updated,
				},
			})

			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(st.PullRequest)
			}
			verb := "opened"
			if ref.Updated {
				verb = "updated"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "pr %s: %s\n", verb, ref.URL)
			return nil
		},
	}
	cmd.Flags().BoolVar(&draft, "draft", false, "open as a draft PR where the platform CLI supports it")
	cmd.Flags().BoolVar(&refuseUpdate, "refuse-update", false, "refuse to update an existing PR; exit 33 instead")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the pull_request record as JSON on stdout")
	return cmd
}

func resolveRemote(repoRoot string) (url, name string, err error) {
	name = "origin"
	out, berr := exec.Command("git", "-C", repoRoot, "remote", "get-url", name).Output()
	if berr != nil {
		return "", "", fmt.Errorf("no %q remote configured (%v)", name, berr)
	}
	return strings.TrimSpace(string(out)), name, nil
}

func sessionBranchHasCommits(repoRoot string, st *session.State) error {
	if st.BaseSHA == "" {
		return fmt.Errorf("session has no recorded base_sha")
	}
	cmd := exec.Command("git", "-C", repoRoot, "rev-list", "--count",
		st.BaseSHA+".."+strings.TrimPrefix(st.SessionBranch, "refs/heads/"))
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("rev-list failed: %v", err)
	}
	if strings.TrimSpace(string(out)) == "0" {
		return fmt.Errorf("session branch has no commits beyond base")
	}
	return nil
}

func derivePRBody(sessionDir string, st *session.State) (title, body string) {
	title = "bender session " + st.SessionID
	body = "Opened from bender session " + st.SessionID + " (branch " +
		strings.TrimPrefix(st.SessionBranch, "refs/heads/") + ")."
	summaryPath := filepath.Join(sessionDir, "summary.md")
	if raw, err := os.ReadFile(summaryPath); err == nil && len(raw) > 0 {
		// First non-empty line becomes the title; full file is the body.
		for _, line := range strings.Split(string(raw), "\n") {
			t := strings.TrimSpace(strings.TrimLeft(line, "# "))
			if t != "" {
				title = t
				break
			}
		}
		body = string(raw)
	}
	return title, body
}

// PRAdapterFactoryForTest installs a custom adapter registry for the duration
// of a test. Returns a restorer the test should defer to reset state.
func PRAdapterFactoryForTest(factory func() []pr.Adapter) (restore func()) {
	prev := prAdapterFactory
	prAdapterFactory = factory
	return func() { prAdapterFactory = prev }
}

var _ io.Writer = (*os.File)(nil) // silence unused import guard if body grows.

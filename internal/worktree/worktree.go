package worktree

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mayckol/ai-bender/internal/event"
	"github.com/mayckol/ai-bender/internal/session"
)

// Exit codes from contracts/cli.md.
const (
	ExitGitUnavailable      = 10
	ExitRepoIncompatible    = 11
	ExitPlacementRefused    = 12
	ExitBranchCollision     = 13
	ExitSessionMissing      = 20
	ExitRefusedActive       = 21
	ExitPRIneligible        = 30
	ExitPRNoAdapter         = 31
	ExitPRAdapterFailed     = 32
	ExitPRUpdateRefused     = 33
)

// ErrBranchCollision is returned (wrapped) when the derived session branch
// name already exists as a local branch.
var ErrBranchCollision = errors.New("branch collision")

// ErrRepoIncompatible is returned (wrapped) when the repo is in a state that
// forbids `git worktree add` (mid-rebase, mid-merge, mid-cherry-pick, bare).
var ErrRepoIncompatible = errors.New("repo incompatible with worktree")

// ErrSessionNotFound is returned when the on-disk session directory is
// missing.
var ErrSessionNotFound = errors.New("session not found")

// ErrActiveSession is returned when a destructive operation is refused
// because the session is still `active` / `running`.
var ErrActiveSession = errors.New("session is active")

// CreateInput is the input to Create.
type CreateInput struct {
	RepoRoot   string    // absolute path of the main repo working tree
	SessionID  string    // unique session identifier (already validated)
	BaseBranch string    // optional; when empty, Create uses the current branch
	Command    string    // e.g. "bender worktree create" — recorded as state.command
	Runner     GitRunner // git invoker; required
	Now        func() time.Time

	// Optional workflow linkage (feature 007). When both are empty, the
	// session is its own workflow; state.json records no workflow metadata.
	WorkflowID              string
	WorkflowParentSessionID string
}

// CreateOutput describes the newly-materialised session.
type CreateOutput struct {
	SessionID     string
	WorktreePath  string
	Branch        string        // e.g. "bender/session/<id>"
	BaseBranch    string
	BaseSHA       string
	SessionDir    string        // .bender/sessions/<id>/
	CreatedAt     time.Time
}

// Create materialises a session-owned git worktree and persists its metadata.
//
// The operation is strictly ordered so that any refusal path leaves the repo
// state untouched:
//
//  1. Prereqs: git available, repo usable (not bare, not mid-rebase/merge).
//  2. Base branch resolution + SHA lookup.
//  3. Branch-collision check.
//  4. Placement validation against the resolved worktree root.
//  5. `git worktree add -b <branch> <path> <base-sha>` — first side effect.
//  6. Persist `state.json` (v2) to .bender/sessions/<id>/state.json.
//  7. Append `worktree_created` event to events.jsonl.
func Create(ctx context.Context, in CreateInput) (*CreateOutput, error) {
	if in.Runner == nil {
		return nil, fmt.Errorf("worktree: runner is required")
	}
	if in.Now == nil {
		in.Now = time.Now
	}
	if err := ValidateSessionID(in.SessionID); err != nil {
		return nil, err
	}

	if err := ProbeGit(ctx, in.Runner); err != nil {
		return nil, err
	}

	if err := checkRepoUsable(ctx, in.Runner, in.RepoRoot); err != nil {
		return nil, err
	}

	baseBranch := in.BaseBranch
	if baseBranch == "" {
		resolved, err := currentBranch(ctx, in.Runner, in.RepoRoot)
		if err != nil {
			return nil, err
		}
		baseBranch = resolved
	}
	baseSHA, err := resolveBaseSHA(ctx, in.Runner, in.RepoRoot, baseBranch)
	if err != nil {
		return nil, err
	}

	branch := BranchName(in.SessionID)
	exists, err := BranchExists(ctx, in.Runner, in.RepoRoot, branch)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("%w: %s", ErrBranchCollision, branch)
	}

	root, err := ResolveRoot(in.RepoRoot)
	if err != nil {
		return nil, err
	}
	wtPath := filepath.Join(root, in.SessionID)
	if err := ValidatePlacement(in.RepoRoot, wtPath); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		return nil, fmt.Errorf("worktree: mkdir root: %w", err)
	}

	_, stderr, err := in.Runner.Run(ctx, in.RepoRoot,
		"worktree", "add", "-b", branch, wtPath, baseSHA)
	if err != nil {
		return nil, fmt.Errorf("worktree: git worktree add: %w: %s",
			err, strings.TrimSpace(string(stderr)))
	}

	createdAt := in.Now().UTC()
	sessionDir := filepath.Join(in.RepoRoot, ".bender", "sessions", in.SessionID)
	state := &session.State{
		SchemaVersion: session.SchemaVersion,
		SessionID:     in.SessionID,
		Command:       in.Command,
		StartedAt:     createdAt,
		Status:        "running",
		SessionBranch: "refs/heads/" + branch,
		BaseBranch:    baseBranch,
		BaseSHA:       baseSHA,
		Worktree: session.Worktree{
			Path:      wtPath,
			Status:    session.WorktreeActive,
			CreatedAt: createdAt,
		},
		WorkflowID:              in.WorkflowID,
		WorkflowParentSessionID: in.WorkflowParentSessionID,
	}
	if err := session.SaveState(sessionDir, state); err != nil {
		return nil, err
	}
	ev := &event.Event{
		SchemaVersion: event.SchemaVersion,
		SessionID:     in.SessionID,
		Timestamp:     createdAt,
		Actor:         event.Actor{Kind: event.ActorOrchestrator, Name: "bender-worktree"},
		Type:          event.TypeWorktreeCreated,
		Payload: map[string]any{
			"path":        wtPath,
			"branch":      branch,
			"base_branch": baseBranch,
			"base_sha":    baseSHA,
		},
	}
	if err := appendEvent(sessionDir, ev); err != nil {
		return nil, err
	}

	return &CreateOutput{
		SessionID:    in.SessionID,
		WorktreePath: wtPath,
		Branch:       branch,
		BaseBranch:   baseBranch,
		BaseSHA:      baseSHA,
		SessionDir:   sessionDir,
		CreatedAt:    createdAt,
	}, nil
}

// checkRepoUsable verifies the repo at repoRoot is non-bare and not mid-rebase,
// -merge, or -cherry-pick. Returns ErrRepoIncompatible (wrapped) on failure.
func checkRepoUsable(ctx context.Context, runner GitRunner, repoRoot string) error {
	stdout, stderr, err := runner.Run(ctx, repoRoot, "rev-parse", "--is-bare-repository")
	if err != nil {
		return fmt.Errorf("%w: rev-parse failed: %s",
			ErrRepoIncompatible, strings.TrimSpace(string(stderr)))
	}
	if strings.TrimSpace(string(stdout)) == "true" {
		return fmt.Errorf("%w: repo is bare", ErrRepoIncompatible)
	}
	for _, marker := range []string{"rebase-merge", "rebase-apply", "MERGE_HEAD", "CHERRY_PICK_HEAD"} {
		if _, err := os.Stat(filepath.Join(repoRoot, ".git", marker)); err == nil {
			return fmt.Errorf("%w: repo is mid-%s", ErrRepoIncompatible, marker)
		}
	}
	return nil
}

func currentBranch(ctx context.Context, runner GitRunner, repoRoot string) (string, error) {
	stdout, stderr, err := runner.Run(ctx, repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("%w: cannot resolve HEAD: %s",
			ErrRepoIncompatible, strings.TrimSpace(string(stderr)))
	}
	name := strings.TrimSpace(string(stdout))
	if name == "HEAD" {
		return "", fmt.Errorf("%w: detached HEAD", ErrRepoIncompatible)
	}
	return name, nil
}

func resolveBaseSHA(ctx context.Context, runner GitRunner, repoRoot, ref string) (string, error) {
	stdout, stderr, err := runner.Run(ctx, repoRoot, "rev-parse", "--verify", ref)
	if err != nil {
		return "", fmt.Errorf("%w: cannot resolve base %q: %s",
			ErrRepoIncompatible, ref, strings.TrimSpace(string(stderr)))
	}
	return strings.TrimSpace(string(stdout)), nil
}

// appendEvent appends one event line to .bender/sessions/<id>/events.jsonl.
// Keeps SaveState-then-event ordering (state is authoritative; absent event
// lines are tolerated by sinks).
func appendEvent(sessionDir string, ev *event.Event) error {
	line, err := ev.MarshalJSONLine()
	if err != nil {
		return fmt.Errorf("worktree: marshal event: %w", err)
	}
	p := filepath.Join(sessionDir, "events.jsonl")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("worktree: open events.jsonl: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("worktree: write event: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("worktree: sync events.jsonl: %w", err)
	}
	return nil
}

// Row is one entry returned by List.
type Row struct {
	SessionID    string
	State        *session.State // loaded state; nil if state.json missing
	WorktreePath string         // absolute, may not exist on disk
	Branch       string
	PresentOnDisk bool
}

// List walks `.bender/sessions/*/state.json` and returns one Row per session.
// Performance budget: one stat() per row, no per-row `git worktree list`.
func List(repoRoot string) ([]Row, error) {
	dir := filepath.Join(repoRoot, ".bender", "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("worktree: read sessions dir: %w", err)
	}
	var rows []Row
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sd := filepath.Join(dir, e.Name())
		st, err := session.LoadState(sd)
		if err != nil {
			// Missing/unparseable state.json — surface as a row with nil state.
			rows = append(rows, Row{SessionID: e.Name()})
			continue
		}
		r := Row{
			SessionID:    st.SessionID,
			State:        st,
			WorktreePath: st.Worktree.Path,
			Branch:       strings.TrimPrefix(st.SessionBranch, "refs/heads/"),
		}
		if r.WorktreePath != "" {
			if _, err := os.Stat(r.WorktreePath); err == nil {
				r.PresentOnDisk = true
			}
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].SessionID < rows[j].SessionID })
	return rows, nil
}

// RemoveInput is the input to Remove.
type RemoveInput struct {
	RepoRoot  string
	SessionID string
	Force     bool
	Runner    GitRunner
	Now       func() time.Time
}

// RemoveResult distinguishes an ordinary cleanup from a reconcile-missing.
type RemoveResult struct {
	Reason string // "cleanup" | "reconciled-missing"
}

// Remove tears down a session's worktree — directory + git metadata — while
// leaving the session branch and the audit trail intact (FR-008).
//
// Refuses on `active` sessions unless the caller has already terminated them
// (the refusal is part of the contract; see exit code 21 in cli.md).
func Remove(ctx context.Context, in RemoveInput) (*RemoveResult, error) {
	if in.Runner == nil {
		return nil, fmt.Errorf("worktree: runner is required")
	}
	if in.Now == nil {
		in.Now = time.Now
	}
	sessionDir := filepath.Join(in.RepoRoot, ".bender", "sessions", in.SessionID)
	st, err := session.LoadState(sessionDir)
	if err != nil {
		if errors.Is(err, session.ErrNoState) {
			return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, in.SessionID)
		}
		return nil, err
	}
	if st.Status == "running" {
		return nil, fmt.Errorf("%w: %s", ErrActiveSession, in.SessionID)
	}

	reason := "cleanup"
	wtPath := st.Worktree.Path
	if wtPath == "" {
		// Legacy v1 session — nothing to remove; treat as a no-op reconcile.
		reason = "reconciled-missing"
	} else if _, err := os.Stat(wtPath); errors.Is(err, os.ErrNotExist) {
		// Directory is gone; prune git metadata to keep `git worktree list` honest.
		reason = "reconciled-missing"
		_, _, _ = in.Runner.Run(ctx, in.RepoRoot, "worktree", "prune")
	} else {
		args := []string{"worktree", "remove"}
		if in.Force {
			args = append(args, "--force")
		}
		args = append(args, wtPath)
		_, stderr, err := in.Runner.Run(ctx, in.RepoRoot, args...)
		if err != nil {
			return nil, fmt.Errorf("worktree: git worktree remove: %w: %s",
				err, strings.TrimSpace(string(stderr)))
		}
	}

	now := in.Now().UTC()
	st.Worktree.Status = session.WorktreeRemoved
	st.Worktree.RemovedAt = &now
	if err := session.SaveState(sessionDir, st); err != nil {
		return nil, err
	}
	ev := &event.Event{
		SchemaVersion: event.SchemaVersion,
		SessionID:     in.SessionID,
		Timestamp:     now,
		Actor:         event.Actor{Kind: event.ActorOrchestrator, Name: "bender-worktree"},
		Type:          event.TypeWorktreeRemoved,
		Payload: map[string]any{
			"path":   wtPath,
			"reason": reason,
		},
	}
	if err := appendEvent(sessionDir, ev); err != nil {
		return nil, err
	}
	return &RemoveResult{Reason: reason}, nil
}

// PruneInput is the input to Prune.
type PruneInput struct {
	RepoRoot   string
	OlderThan  time.Duration // 0 = prune every eligible session regardless of age
	Runner     GitRunner
	Now        func() time.Time
}

// PruneSummary is the aggregate outcome of a Prune call.
type PruneSummary struct {
	Removed     int
	Reconciled  int
	Skipped     int
	FailedSessions []string
}

// Prune iterates sessions with a terminal status and removes their worktrees.
// OlderThan filters by completed_at (zero or missing = always eligible).
func Prune(ctx context.Context, in PruneInput) (*PruneSummary, error) {
	if in.Now == nil {
		in.Now = time.Now
	}
	rows, err := List(in.RepoRoot)
	if err != nil {
		return nil, err
	}
	now := in.Now().UTC()
	summary := &PruneSummary{}
	for _, r := range rows {
		if r.State == nil {
			summary.Skipped++
			continue
		}
		switch r.State.Status {
		case "completed", "failed", "awaiting_confirm":
		default:
			summary.Skipped++
			continue
		}
		if r.State.Worktree.Status == session.WorktreeRemoved {
			summary.Skipped++
			continue
		}
		if in.OlderThan > 0 && !r.State.CompletedAt.IsZero() &&
			now.Sub(r.State.CompletedAt) < in.OlderThan {
			summary.Skipped++
			continue
		}
		res, err := Remove(ctx, RemoveInput{
			RepoRoot:  in.RepoRoot,
			SessionID: r.SessionID,
			Runner:    in.Runner,
			Now:       in.Now,
		})
		if err != nil {
			summary.FailedSessions = append(summary.FailedSessions, r.SessionID)
			continue
		}
		if res.Reason == "reconciled-missing" {
			summary.Reconciled++
		} else {
			summary.Removed++
		}
	}
	return summary, nil
}

// AppendEvent is a public convenience for external callers (e.g. cmd/bender)
// that need to emit one of the feature's event kinds against a session.
func AppendEvent(sessionDir string, ev *event.Event) error { return appendEvent(sessionDir, ev) }

// MarkMissing records a worktree.missing event and transitions the session to
// failed. Callers (the orchestrator skill, an external watcher, or a server
// sink) invoke this when they detect that a live session's worktree directory
// has vanished out-of-band. The main repo is not touched.
func MarkMissing(repoRoot, sessionID string, now func() time.Time) error {
	if now == nil {
		now = time.Now
	}
	sessionDir := filepath.Join(repoRoot, ".bender", "sessions", sessionID)
	st, err := session.LoadState(sessionDir)
	if err != nil {
		if errors.Is(err, session.ErrNoState) {
			return fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
		}
		return err
	}
	ts := now().UTC()
	st.Worktree.Status = session.WorktreeMissing
	st.Status = "failed"
	if st.CompletedAt.IsZero() {
		st.CompletedAt = ts
	}
	if err := session.SaveState(sessionDir, st); err != nil {
		return err
	}
	ev := &event.Event{
		SchemaVersion: event.SchemaVersion,
		SessionID:     sessionID,
		Timestamp:     ts,
		Actor:         event.Actor{Kind: event.ActorOrchestrator, Name: "bender-worktree"},
		Type:          event.TypeWorktreeMissing,
		Payload:       map[string]any{"path": st.Worktree.Path},
	}
	return appendEvent(sessionDir, ev)
}

// MarshalForScript renders state as pretty JSON so fallback-parity tests can
// diff it against the Bash/PowerShell scripts' output.
func MarshalForScript(state *session.State) ([]byte, error) {
	return json.MarshalIndent(state, "", "  ")
}

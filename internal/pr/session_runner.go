package pr

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mayckol/ai-bender/internal/session"
)

// SessionRunOptions carries the inputs for RunForSession. A struct keeps the
// signature within the three-argument project convention and makes feature
// 007's extension-hook call site self-documenting.
type SessionRunOptions struct {
	// ProjectRoot is the repo root whose .bender/sessions tree contains the
	// session.
	ProjectRoot string
	// SessionID identifies the session to open (or update) a PR for.
	SessionID string
	// Draft, when true, opens the PR as a draft where the platform supports it.
	Draft bool
	// RefuseUpdate, when true, returns ErrExistingPRPresent rather than
	// refreshing the body of an already-open PR.
	RefuseUpdate bool
	// Adapters optionally overrides the default set (DefaultAdapters). Tests
	// inject fakes through this field.
	Adapters []Adapter
	// Exec optionally overrides the git/remote executor. When nil, os/exec is
	// used directly for the git helper calls.
	Exec ExecRunner
}

// SessionRunResult reports what RunForSession did.
type SessionRunResult struct {
	Skipped  bool   // true when the session was ineligible and no PR action ran
	Reason   string // short human-readable label when Skipped is true
	PRURL    string // set when a PR was opened or refreshed
	Updated  bool   // true when an existing PR was refreshed (not newly opened)
	Adapter  string // adapter name that handled the action
}

// Common skip reasons.
const (
	ReasonSessionMissing = "session_missing"
	ReasonLegacySession  = "legacy_session"
	ReasonActive         = "session_active"
	ReasonNoCommits      = "branch_empty"
)

// RunForSession performs the same work as `bender session pr` but returns
// typed errors instead of calling os.Exit, so it can be invoked from hooks
// and tests. Eligibility gates are surfaced through SessionRunResult.Skipped
// rather than errors — a skip is an expected no-op, not a failure.
func RunForSession(ctx context.Context, opts SessionRunOptions) (SessionRunResult, error) {
	if opts.ProjectRoot == "" {
		return SessionRunResult{}, errors.New("pr: ProjectRoot is required")
	}
	if opts.SessionID == "" {
		return SessionRunResult{}, errors.New("pr: SessionID is required")
	}

	sessionDir := filepath.Join(opts.ProjectRoot, ".bender", "sessions", opts.SessionID)
	st, err := session.LoadState(sessionDir)
	if err != nil {
		if errors.Is(err, session.ErrNoState) {
			return SessionRunResult{Skipped: true, Reason: ReasonSessionMissing}, nil
		}
		return SessionRunResult{}, err
	}
	if st.IsLegacy() {
		return SessionRunResult{Skipped: true, Reason: ReasonLegacySession}, nil
	}
	if st.Status == "running" {
		return SessionRunResult{Skipped: true, Reason: ReasonActive}, nil
	}

	exe := opts.Exec
	if exe == nil {
		exe = SystemExecRunner{}
	}

	if err := sessionBranchHasCommits(opts.ProjectRoot, st); err != nil {
		return SessionRunResult{Skipped: true, Reason: ReasonNoCommits}, nil
	}

	remoteURL, remoteName, err := resolveRemoteExec(opts.ProjectRoot)
	if err != nil {
		return SessionRunResult{}, err
	}

	adapters := opts.Adapters
	if adapters == nil {
		adapters = DefaultAdapters(exe)
	}
	adapter, err := SelectAdapter(adapters, remoteURL)
	if err != nil {
		return SessionRunResult{}, err
	}
	if err := adapter.AuthCheck(ctx); err != nil {
		return SessionRunResult{}, err
	}

	shortBranch := strings.TrimPrefix(st.SessionBranch, "refs/heads/")
	if err := adapter.Push(ctx, PushInput{
		RepoRoot:     opts.ProjectRoot,
		Remote:       remoteName,
		LocalBranch:  st.SessionBranch,
		RemoteBranch: shortBranch,
	}); err != nil {
		return SessionRunResult{}, err
	}

	title := "bender session " + st.SessionID
	body := "Opened from bender session " + st.SessionID + " (branch " + shortBranch + ")."

	ref, err := adapter.OpenOrUpdate(ctx, OpenArgs{
		RepoRoot:     opts.ProjectRoot,
		Remote:       remoteName,
		Base:         st.BaseBranch,
		Head:         shortBranch,
		Title:        title,
		Body:         body,
		Draft:        opts.Draft,
		RefuseUpdate: opts.RefuseUpdate,
	})
	if err != nil {
		return SessionRunResult{}, err
	}

	return SessionRunResult{
		PRURL:   ref.URL,
		Updated: ref.Updated,
		Adapter: adapter.Name(),
	}, nil
}

// resolveRemoteExec is a thin wrapper used from the library so session_pr.go
// and this runner share one implementation for "find the origin URL".
func resolveRemoteExec(repoRoot string) (url, name string, err error) {
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
	short := strings.TrimPrefix(st.SessionBranch, "refs/heads/")
	out, err := exec.Command("git", "-C", repoRoot, "rev-list", "--count",
		st.BaseSHA+".."+short).Output()
	if err != nil {
		return fmt.Errorf("rev-list failed: %v", err)
	}
	if strings.TrimSpace(string(out)) == "0" {
		return fmt.Errorf("session branch has no commits beyond base")
	}
	return nil
}

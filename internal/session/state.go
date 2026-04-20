// Package session reads and exports the on-disk session artifacts produced by Claude Code
// (or any client) when executing a slash command. Sessions live under
// .bender/sessions/<id>/ and contain state.json + events.jsonl.
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// SchemaVersion is the latest state.json schema version accepted by new writers.
// v1 files continue to load — validateState accepts both 1 and 2.
const SchemaVersion = 2

// StatusAwaitingClarification is the session status set while a /plan run is
// paused waiting for clarification answers (interactive mode), or after a
// non-interactive strict-mode run aborted before writing the rest of the plan
// set. Added for feature 006-plan-clarifications. Additive enum value;
// schema_version is not bumped.
const StatusAwaitingClarification = "awaiting_clarification"

// WorktreeStatus tracks the lifecycle of the per-session worktree directory.
type WorktreeStatus string

const (
	WorktreeActive    WorktreeStatus = "active"
	WorktreeCompleted WorktreeStatus = "completed"
	WorktreeAborted   WorktreeStatus = "aborted"
	WorktreeRemoved   WorktreeStatus = "removed"
	WorktreeMissing   WorktreeStatus = "missing"
)

// Worktree describes the per-session git worktree materialised at session start.
// Populated for v2 sessions only; v1 sessions load with a zero Worktree value
// and are surfaced by callers as "legacy" via State.IsLegacy.
type Worktree struct {
	Path      string         `json:"path"`
	Status    WorktreeStatus `json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	RemovedAt *time.Time     `json:"removed_at,omitempty"`
}

// PullRequest is the optional snapshot recorded when the user invokes
// `bender session pr <id>`. Absent until the first successful invocation.
// Bender never polls the platform to refresh fields here; LastInvokedAt is
// updated on every successful re-invocation (the PR body is refreshed),
// OpenedAt is immutable.
type PullRequest struct {
	Remote         string    `json:"remote"`
	RemoteURL      string    `json:"remote_url"`
	BranchOnRemote string    `json:"branch_on_remote"`
	URL            string    `json:"pr_url"`
	OpenedAt       time.Time `json:"opened_at"`
	LastInvokedAt  time.Time `json:"last_invoked_at,omitzero"`
	Adapter        string    `json:"adapter"`
}

// State mirrors the on-disk state.json snapshot.
//
// v1 fields remain unchanged. v2 adds Worktree, SessionBranch, BaseBranch,
// BaseSHA, and the optional PullRequest. v1 files load cleanly into v2 with
// zero-valued new fields; callers check SchemaVersion or IsLegacy when they
// need to distinguish modes.
type State struct {
	SchemaVersion   int       `json:"schema_version"`
	SessionID       string    `json:"session_id"`
	Command         string    `json:"command"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at,omitzero"`
	Status          string    `json:"status"` // running | awaiting_confirm | completed | failed
	SourceArtifacts []string  `json:"source_artifacts,omitempty"`
	SkillsInvoked   []string  `json:"skills_invoked,omitempty"`
	FilesChanged    int       `json:"files_changed,omitempty"`
	FindingsCount   int       `json:"findings_count,omitempty"`

	Worktree      Worktree     `json:"worktree,omitzero"`
	SessionBranch string       `json:"session_branch,omitempty"`
	BaseBranch    string       `json:"base_branch,omitempty"`
	BaseSHA       string       `json:"base_sha,omitempty"`
	PullRequest   *PullRequest `json:"pull_request,omitempty"`

	// ClarificationsArtifact points at the in-progress
	// .bender/artifacts/plan/clarifications-<ts>.md while Status is
	// "awaiting_clarification". Empty otherwise. Added for feature
	// 006-plan-clarifications.
	ClarificationsArtifact string `json:"clarifications_artifact,omitempty"`

	// WorkflowID groups consecutive sessions belonging to the same feature
	// (e.g. /tdd followed by /ghu). Absent for standalone sessions. Added for
	// feature 007-flow-scout-init-fixes. Additive; schema_version is not bumped.
	WorkflowID string `json:"workflow_id,omitempty"`

	// WorkflowParentSessionID points at the prior session in the same workflow,
	// when this session inherits its WorkflowID. Empty on the first session of
	// a workflow. Added for 007-flow-scout-init-fixes.
	WorkflowParentSessionID string `json:"workflow_parent_session_id,omitempty"`
}

// ErrNoState is returned when state.json is missing from a session directory.
var ErrNoState = errors.New("session: state.json not found")

// IsLegacy reports whether this state was loaded from a v1 state.json (no
// worktree metadata). UI surfaces use this to label rows; cleanup code skips
// legacy sessions because there is nothing on disk to remove.
func (s *State) IsLegacy() bool {
	return s.SchemaVersion < 2 || s.Worktree.Path == ""
}

// LoadState reads and parses state.json from sessionDir. It is intentionally
// permissive — it performs no version rejection and no field-presence checks
// beyond what Go's JSON unmarshal already enforces. Callers that need strict
// validation use Validate.
func LoadState(sessionDir string) (*State, error) {
	data, err := os.ReadFile(filepath.Join(sessionDir, "state.json"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNoState
		}
		return nil, fmt.Errorf("session: read state: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("session: parse state: %w", err)
	}
	return &s, nil
}

// SaveState atomically writes state.json to sessionDir via temp-file + rename.
// Creates sessionDir if it does not exist. Used by bender's Go-side writers
// (e.g. `bender worktree remove` recording the removed_at timestamp).
func SaveState(sessionDir string, s *State) error {
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return fmt.Errorf("session: mkdir session dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("session: marshal state: %w", err)
	}
	data = append(data, '\n')
	tmp := filepath.Join(sessionDir, "state.json.tmp")
	final := filepath.Join(sessionDir, "state.json")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("session: write temp state: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return fmt.Errorf("session: rename state: %w", err)
	}
	return nil
}

package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func plantStateFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadState_V1_StillLoads_AsLegacy(t *testing.T) {
	body := `{
  "schema_version": 1,
  "session_id": "legacy-session",
  "command": "/ghu",
  "started_at": "2026-04-16T14:03:22Z",
  "status": "running"
}
`
	s, err := LoadState(plantStateFile(t, body))
	if err != nil {
		t.Fatalf("LoadState v1: %v", err)
	}
	if s.SchemaVersion != 1 {
		t.Fatalf("schema_version: got %d want 1", s.SchemaVersion)
	}
	if !s.IsLegacy() {
		t.Fatalf("expected IsLegacy=true for v1 state")
	}
	if s.Worktree.Path != "" {
		t.Fatalf("v1 state must have empty Worktree.Path, got %q", s.Worktree.Path)
	}
}

func TestLoadState_V2_PopulatesWorktreeFields(t *testing.T) {
	body := `{
  "schema_version": 2,
  "session_id": "019a3b4c-e1f2-7c8d-9a0b-1c2d3e4f5061",
  "command": "/ghu",
  "started_at": "2026-04-18T14:30:22Z",
  "status": "running",
  "session_branch": "refs/heads/bender/session/019a3b4c-e1f2-7c8d-9a0b-1c2d3e4f5061",
  "base_branch": "main",
  "base_sha": "9f0a1b2c3d4e5f60718293a4b5c6d7e8f9012345",
  "worktree": {
    "path": "/repo/.bender/worktrees/019a3b4c-e1f2-7c8d-9a0b-1c2d3e4f5061",
    "status": "active",
    "created_at": "2026-04-18T14:30:22Z"
  }
}
`
	s, err := LoadState(plantStateFile(t, body))
	if err != nil {
		t.Fatalf("LoadState v2: %v", err)
	}
	if s.IsLegacy() {
		t.Fatalf("expected IsLegacy=false for v2 state")
	}
	if s.Worktree.Status != WorktreeActive {
		t.Fatalf("worktree.status: got %q want active", s.Worktree.Status)
	}
	if s.SessionBranch == "" || s.BaseBranch == "" || s.BaseSHA == "" {
		t.Fatalf("v2 fields must be populated: %+v", s)
	}
}

func TestValidateState_AcceptsV1(t *testing.T) {
	s := &State{
		SchemaVersion: 1,
		SessionID:     "s1",
		Command:       "/plan",
		StartedAt:     time.Now().UTC(),
		Status:        "running",
	}
	msgs := validateState(s)
	if len(msgs) != 0 {
		t.Fatalf("expected zero violations for minimal v1, got %v", msgs)
	}
}

func TestValidateState_V2_RequiresWorktreeFields(t *testing.T) {
	s := &State{
		SchemaVersion: 2,
		SessionID:     "s2",
		Command:       "/ghu",
		StartedAt:     time.Now().UTC(),
		Status:        "running",
	}
	msgs := validateState(s)
	joined := strings.Join(msgs, "\n")
	for _, want := range []string{
		"worktree.path is required",
		"session_branch is required",
		"base_branch is required",
		"base_sha is required",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("validateState(v2): want violation containing %q, got:\n%s", want, joined)
		}
	}
}

func TestSaveState_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	original := &State{
		SchemaVersion: 2,
		SessionID:     "rt-1",
		Command:       "bender worktree create",
		StartedAt:     time.Date(2026, 4, 18, 14, 30, 22, 0, time.UTC),
		Status:        "running",
		SessionBranch: "refs/heads/bender/session/rt-1",
		BaseBranch:    "main",
		BaseSHA:       "0000000000000000000000000000000000000001",
		Worktree: Worktree{
			Path:      "/tmp/rt-1",
			Status:    WorktreeActive,
			CreatedAt: time.Date(2026, 4, 18, 14, 30, 22, 0, time.UTC),
		},
	}
	if err := SaveState(dir, original); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	loaded, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState after SaveState: %v", err)
	}
	if loaded.Worktree.Path != original.Worktree.Path {
		t.Fatalf("Worktree.Path: got %q want %q", loaded.Worktree.Path, original.Worktree.Path)
	}
	if loaded.SessionBranch != original.SessionBranch {
		t.Fatalf("SessionBranch: got %q want %q", loaded.SessionBranch, original.SessionBranch)
	}
}

package session

import (
	"encoding/json"
	"strings"
	"testing"
)

// The full JSON-schema validator is overkill for the three things we actually
// need to enforce on v2 state.json:
//
//  1. schema_version is 1 or 2.
//  2. When schema_version == 2, worktree/session_branch/base_branch/base_sha
//     are present.
//  3. pull_request, when present, carries its required fields.
//
// These are exactly the rules encoded in contracts/state-v2.schema.yaml. We
// check them here against canonical fixtures without pulling in a draft-07
// validator dependency.

const fixtureV1 = `{
  "schema_version": 1,
  "session_id": "legacy",
  "command": "/ghu",
  "started_at": "2026-04-16T14:03:22Z",
  "status": "running"
}`

const fixtureV2Valid = `{
  "schema_version": 2,
  "session_id": "019a",
  "command": "/ghu",
  "started_at": "2026-04-18T14:30:22Z",
  "status": "running",
  "session_branch": "refs/heads/bender/session/019a",
  "base_branch": "main",
  "base_sha": "9f0a1b2c3d4e5f60718293a4b5c6d7e8f9012345",
  "worktree": {
    "path": "/repo/.bender/worktrees/019a",
    "status": "active",
    "created_at": "2026-04-18T14:30:22Z"
  }
}`

const fixtureV2MissingWorktree = `{
  "schema_version": 2,
  "session_id": "019b",
  "command": "/ghu",
  "started_at": "2026-04-18T14:30:22Z",
  "status": "running"
}`

const fixtureV2WithPR = `{
  "schema_version": 2,
  "session_id": "019c",
  "command": "/ghu",
  "started_at": "2026-04-18T14:30:22Z",
  "status": "completed",
  "completed_at": "2026-04-18T14:40:22Z",
  "session_branch": "refs/heads/bender/session/019c",
  "base_branch": "main",
  "base_sha": "9f0a1b2c3d4e5f60718293a4b5c6d7e8f9012345",
  "worktree": {
    "path": "/repo/.bender/worktrees/019c",
    "status": "completed",
    "created_at": "2026-04-18T14:30:22Z"
  },
  "pull_request": {
    "remote": "origin",
    "remote_url": "https://github.com/acme/repo",
    "branch_on_remote": "bender/session/019c",
    "pr_url": "https://github.com/acme/repo/pull/1",
    "opened_at": "2026-04-18T14:45:22Z",
    "adapter": "gh"
  }
}`

func TestSchemaFixtures_V1_ValidatesClean(t *testing.T) {
	var s State
	if err := json.Unmarshal([]byte(fixtureV1), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	msgs := validateState(&s)
	if len(msgs) != 0 {
		t.Fatalf("v1 fixture must validate clean, got %v", msgs)
	}
}

func TestSchemaFixtures_V2Valid_ValidatesClean(t *testing.T) {
	var s State
	if err := json.Unmarshal([]byte(fixtureV2Valid), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	msgs := validateState(&s)
	if len(msgs) != 0 {
		t.Fatalf("valid v2 fixture must validate clean, got %v", msgs)
	}
}

func TestSchemaFixtures_V2MissingWorktree_IsRejected(t *testing.T) {
	var s State
	if err := json.Unmarshal([]byte(fixtureV2MissingWorktree), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	msgs := validateState(&s)
	joined := strings.Join(msgs, "\n")
	if !strings.Contains(joined, "worktree.path is required") {
		t.Fatalf("expected worktree.path violation, got:\n%s", joined)
	}
}

func TestSchemaFixtures_V2WithPR_Parses(t *testing.T) {
	var s State
	if err := json.Unmarshal([]byte(fixtureV2WithPR), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.PullRequest == nil {
		t.Fatal("pull_request should parse to non-nil")
	}
	if s.PullRequest.URL != "https://github.com/acme/repo/pull/1" {
		t.Fatalf("pr_url: got %q", s.PullRequest.URL)
	}
	if s.PullRequest.Adapter != "gh" {
		t.Fatalf("adapter: got %q", s.PullRequest.Adapter)
	}
}

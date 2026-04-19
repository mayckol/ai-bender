package session

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mayckol/ai-bender/internal/event"
)

// Violation describes one schema drift encountered while validating a session.
// Where is a human-readable locator ("state.json" or "events.jsonl:<line>").
type Violation struct {
	Where   string
	Message string
}

func (v Violation) String() string { return v.Where + ": " + v.Message }

// Validate walks a session directory and returns every schema drift it finds.
// An empty slice means the session is contract-compliant.
//
// The checks cover:
//
//  1. state.json — top-level fields required by contracts/session.md.
//  2. events.jsonl — envelope (schema_version, session_id, actor, type) and per-type
//     payload requirements sourced from internal/event.
//  3. cross-consistency — if state.findings_count > 0 the log must contain at least
//     one finding_reported event, and session_id must match state.session_id on every
//     line.
func Validate(sessionDir string) ([]Violation, error) {
	var out []Violation

	state, stateErr := LoadState(sessionDir)
	if stateErr != nil {
		return nil, stateErr
	}
	for _, m := range validateState(state) {
		out = append(out, Violation{Where: "state.json", Message: m})
	}

	eventsPath := filepath.Join(sessionDir, "events.jsonl")
	f, err := os.Open(eventsPath)
	if err != nil {
		return nil, fmt.Errorf("session: open events: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var (
		lineNo        int
		sawFinding    bool
		sawCompletion bool
	)
	for scanner.Scan() {
		lineNo++
		raw := scanner.Bytes()
		if len(strings.TrimSpace(string(raw))) == 0 {
			continue
		}
		ev, parseErr := event.UnmarshalEvent(raw)
		where := fmt.Sprintf("events.jsonl:%d", lineNo)
		if parseErr != nil {
			out = append(out, Violation{Where: where, Message: "unparseable JSON: " + parseErr.Error()})
			continue
		}
		if err := ev.Validate(); err != nil {
			out = append(out, Violation{Where: where, Message: err.Error()})
		}
		if err := ev.ValidatePayload(); err != nil {
			out = append(out, Violation{Where: where, Message: err.Error()})
		}
		if ev.SessionID != "" && ev.SessionID != state.SessionID {
			out = append(out, Violation{
				Where:   where,
				Message: fmt.Sprintf("session_id=%q does not match state.session_id=%q", ev.SessionID, state.SessionID),
			})
		}
		switch ev.Type {
		case event.TypeFindingReported:
			sawFinding = true
		case event.TypeSessionCompleted:
			sawCompletion = true
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("session: scan events: %w", err)
	}

	if state.FindingsCount > 0 && !sawFinding {
		out = append(out, Violation{
			Where:   "events.jsonl",
			Message: fmt.Sprintf("state.findings_count=%d but no finding_reported events emitted", state.FindingsCount),
		})
	}
	if (state.Status == "completed" || state.Status == "awaiting_confirm") && !sawCompletion {
		out = append(out, Violation{
			Where:   "events.jsonl",
			Message: fmt.Sprintf("state.status=%s but no session_completed event emitted", state.Status),
		})
	}

	return out, nil
}

// validateState returns the list of required-field messages for state.json.
// Kept separate so it can be unit-tested without touching disk.
//
// Both schema_version 1 and schema_version 2 are accepted. v2 additionally
// requires the worktree/session_branch/base_branch/base_sha quartet; v1 has
// no such requirement (legacy sessions predate worktree isolation).
func validateState(s *State) []string {
	var msgs []string
	if s.SchemaVersion != 1 && s.SchemaVersion != SchemaVersion {
		msgs = append(msgs, fmt.Sprintf("schema_version=%d (want 1 or %d)", s.SchemaVersion, SchemaVersion))
	}
	if s.SessionID == "" {
		msgs = append(msgs, "session_id is required")
	}
	if s.Command == "" {
		msgs = append(msgs, "command is required")
	}
	if s.StartedAt.IsZero() {
		msgs = append(msgs, "started_at is required")
	}
	switch s.Status {
	case "running", "awaiting_confirm", "completed", "failed":
	default:
		msgs = append(msgs, fmt.Sprintf("status=%q (want running|awaiting_confirm|completed|failed)", s.Status))
	}
	if (s.Status == "awaiting_confirm" || s.Status == "completed" || s.Status == "failed") && s.CompletedAt.IsZero() {
		msgs = append(msgs, fmt.Sprintf("completed_at is required when status=%q", s.Status))
	}
	if s.SchemaVersion == SchemaVersion {
		if s.Worktree.Path == "" {
			msgs = append(msgs, "worktree.path is required for schema_version=2")
		}
		if s.SessionBranch == "" {
			msgs = append(msgs, "session_branch is required for schema_version=2")
		}
		if s.BaseBranch == "" {
			msgs = append(msgs, "base_branch is required for schema_version=2")
		}
		if s.BaseSHA == "" {
			msgs = append(msgs, "base_sha is required for schema_version=2")
		}
	}
	return msgs
}

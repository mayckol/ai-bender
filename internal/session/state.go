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
	"time"
)

// SchemaVersion is the current state.json schema version.
const SchemaVersion = 1

// State mirrors the on-disk state.json snapshot per `contracts/session.md`.
type State struct {
	SchemaVersion   int       `json:"schema_version"`
	SessionID       string    `json:"session_id"`
	Command         string    `json:"command"`
	StartedAt       time.Time `json:"started_at"`
	Status          string    `json:"status"` // running | completed | failed
	SourceArtifacts []string  `json:"source_artifacts,omitempty"`
	SkillsInvoked   []string  `json:"skills_invoked,omitempty"`
	FilesChanged    int       `json:"files_changed,omitempty"`
	FindingsCount   int       `json:"findings_count,omitempty"`
}

// ErrNoState is returned when state.json is missing from a session directory.
var ErrNoState = errors.New("session: state.json not found")

// LoadState reads and parses state.json from sessionDir.
func LoadState(sessionDir string) (*State, error) {
	data, err := os.ReadFile(sessionDir + "/state.json")
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

package session

import (
	"path/filepath"
	"testing"
	"time"
)

func TestState_WorkflowFields_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	dir = filepath.Join(dir, "sess")
	original := State{
		SchemaVersion:           SchemaVersion,
		SessionID:               "parity-x",
		Command:                 "/ghu",
		StartedAt:               time.Now().UTC().Truncate(time.Second),
		Status:                  "running",
		WorkflowID:              "wf-feature-007",
		WorkflowParentSessionID: "tdd-session-0",
	}
	if err := SaveState(dir, &original); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadState(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.WorkflowID != original.WorkflowID {
		t.Errorf("WorkflowID: got %q want %q", loaded.WorkflowID, original.WorkflowID)
	}
	if loaded.WorkflowParentSessionID != original.WorkflowParentSessionID {
		t.Errorf("WorkflowParentSessionID: got %q want %q",
			loaded.WorkflowParentSessionID, original.WorkflowParentSessionID)
	}
}

func TestState_WorkflowFields_AbsentWhenUnset(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sess")
	s := State{
		SchemaVersion: SchemaVersion,
		SessionID:     "solo",
		Command:       "/ghu",
		StartedAt:     time.Now().UTC().Truncate(time.Second),
		Status:        "running",
	}
	if err := SaveState(dir, &s); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadState(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.WorkflowID != "" || loaded.WorkflowParentSessionID != "" {
		t.Fatalf("expected workflow fields empty, got %q / %q",
			loaded.WorkflowID, loaded.WorkflowParentSessionID)
	}
}

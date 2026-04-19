package session

import (
	"encoding/json"
	"testing"
	"time"
)

// TestStatusAwaitingClarification_IsKnown ensures validateState accepts the
// new status added by feature 006-plan-clarifications without requiring a
// completed_at timestamp (the run is paused waiting for input, not terminal).
func TestStatusAwaitingClarification_IsKnown(t *testing.T) {
	if StatusAwaitingClarification != "awaiting_clarification" {
		t.Fatalf("StatusAwaitingClarification literal: got %q want %q",
			StatusAwaitingClarification, "awaiting_clarification")
	}

	s := &State{
		SchemaVersion: 1,
		SessionID:     "s1",
		Command:       "/plan",
		StartedAt:     time.Now().UTC(),
		Status:        StatusAwaitingClarification,
	}
	msgs := validateState(s)
	for _, m := range msgs {
		// Should have zero violations for a v1 awaiting_clarification state.
		t.Errorf("unexpected violation: %s", m)
	}
}

func TestState_ClarificationsArtifact_RoundTrip(t *testing.T) {
	in := State{
		SchemaVersion:          1,
		SessionID:              "s1",
		Command:                "/plan",
		StartedAt:              time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC),
		Status:                 StatusAwaitingClarification,
		ClarificationsArtifact: ".bender/artifacts/plan/clarifications-2026-04-19T10-00-00-000.md",
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out State
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ClarificationsArtifact != in.ClarificationsArtifact {
		t.Fatalf("clarifications_artifact round-trip mismatch: got %q want %q",
			out.ClarificationsArtifact, in.ClarificationsArtifact)
	}
}

func TestState_ClarificationsArtifact_OmitemptyWhenBlank(t *testing.T) {
	s := State{
		SchemaVersion: 1,
		SessionID:     "s1",
		Command:       "/plan",
		StartedAt:     time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC),
		Status:        "running",
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got := string(data); jsonContainsKey(got, "clarifications_artifact") {
		t.Fatalf("expected clarifications_artifact to be omitted when blank, got: %s", got)
	}
}

func jsonContainsKey(haystack, key string) bool {
	for i := 0; i+len(key)+2 < len(haystack); i++ {
		if haystack[i] == '"' && haystack[i+1:i+1+len(key)] == key && haystack[i+1+len(key)] == '"' {
			return true
		}
	}
	return false
}

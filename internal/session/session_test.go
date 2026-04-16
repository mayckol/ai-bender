package session

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const sampleState = `{
  "schema_version": 1,
  "session_id": "2026-04-16T14-03-22-abc",
  "command": "/ghu",
  "started_at": "2026-04-16T14:03:22Z",
  "status": "completed",
  "source_artifacts": [".bender/artifacts/specs/foo.md"],
  "files_changed": 7,
  "findings_count": 2
}
`

const sampleEvents = `{"schema_version":1,"session_id":"2026-04-16T14-03-22-abc","timestamp":"2026-04-16T14:03:22Z","actor":{"kind":"orchestrator","name":"core"},"type":"session_started","payload":{}}
{"schema_version":1,"session_id":"2026-04-16T14-03-22-abc","timestamp":"2026-04-16T14:05:11Z","actor":{"kind":"agent","name":"crafter"},"type":"agent_completed","payload":{"duration_ms":109000}}
{"schema_version":1,"session_id":"2026-04-16T14-03-22-abc","timestamp":"2026-04-16T14:05:30Z","actor":{"kind":"orchestrator","name":"core"},"type":"session_completed","payload":{"status":"completed"}}
`

func plantSession(t *testing.T, projectRoot, id string) string {
	t.Helper()
	dir := filepath.Join(projectRoot, ".bender", "sessions", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(sampleState), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(sampleEvents), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadState_AcceptsValid(t *testing.T) {
	dir := plantSession(t, t.TempDir(), "2026-04-16T14-03-22-abc")
	s, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if s.SessionID != "2026-04-16T14-03-22-abc" {
		t.Fatalf("session id: got %q", s.SessionID)
	}
	if s.Status != "completed" {
		t.Fatalf("status: got %q", s.Status)
	}
}

// TestLoadState_CompletedAt round-trips the optional completed_at field added in v0.3.1.
// Skills emit this when the session reaches a terminal status; absence is tolerated so
// state.json files written by v0.3.0 still parse cleanly.
func TestLoadState_CompletedAt(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".bender", "sessions", "2026-04-16T14-03-22-xyz")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{
  "schema_version": 1,
  "session_id": "2026-04-16T14-03-22-xyz",
  "command": "/cry",
  "started_at": "2026-04-16T14:03:22Z",
  "completed_at": "2026-04-16T14:03:58Z",
  "status": "completed"
}
`
	if err := os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadState(stateDir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	want := time.Date(2026, 4, 16, 14, 3, 58, 0, time.UTC)
	if !s.CompletedAt.Equal(want) {
		t.Fatalf("completed_at: got %v want %v", s.CompletedAt, want)
	}
}

func TestList_ComputesDuration(t *testing.T) {
	root := t.TempDir()
	plantSession(t, root, "2026-04-16T14-03-22-abc")
	listings, err := List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("got %d listings", len(listings))
	}
	wantDur := 2*time.Minute + 8*time.Second
	if listings[0].Duration != wantDur {
		t.Fatalf("duration: got %v want %v", listings[0].Duration, wantDur)
	}
}

func TestCopyEvents_ByteIdentical(t *testing.T) {
	dir := plantSession(t, t.TempDir(), "2026-04-16T14-03-22-abc")
	var buf bytes.Buffer
	if err := CopyEvents(dir, &buf); err != nil {
		t.Fatalf("CopyEvents: %v", err)
	}
	if buf.String() != sampleEvents {
		t.Fatalf("CopyEvents must be byte-identical to events.jsonl")
	}
}

func TestExport_RoundTripStructure(t *testing.T) {
	dir := plantSession(t, t.TempDir(), "2026-04-16T14-03-22-abc")
	var buf bytes.Buffer
	if err := Export(dir, &buf); err != nil {
		t.Fatalf("Export: %v", err)
	}
	var doc ExportDocument
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("export not valid JSON: %v\n%s", err, buf.String())
	}
	if doc.SessionID != "2026-04-16T14-03-22-abc" {
		t.Fatalf("session_id: got %q", doc.SessionID)
	}
	if len(doc.Events) != 3 {
		t.Fatalf("events: got %d want 3", len(doc.Events))
	}
	// SC-009: re-emitting events from the export must produce byte-identical content.
	var rebuilt bytes.Buffer
	for _, e := range doc.Events {
		rebuilt.Write(e)
		rebuilt.WriteByte('\n')
	}
	if rebuilt.String() != sampleEvents {
		t.Fatalf("re-emission not byte-identical:\nGOT:\n%s\nWANT:\n%s", rebuilt.String(), sampleEvents)
	}
}

func TestList_EmptyRootReturnsNil(t *testing.T) {
	listings, err := List(t.TempDir())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if listings != nil {
		t.Fatalf("expected nil for empty root, got %v", listings)
	}
}

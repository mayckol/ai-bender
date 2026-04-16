package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const plantedState = `{
  "schema_version": 1,
  "session_id": "2026-04-16T14-03-22-zzz",
  "command": "/ghu",
  "started_at": "2026-04-16T14:03:22Z",
  "status": "completed",
  "files_changed": 7,
  "findings_count": 2
}
`

const plantedEvents = `{"schema_version":1,"session_id":"2026-04-16T14-03-22-zzz","timestamp":"2026-04-16T14:03:22Z","actor":{"kind":"orchestrator","name":"core"},"type":"session_started","payload":{}}
{"schema_version":1,"session_id":"2026-04-16T14-03-22-zzz","timestamp":"2026-04-16T14:05:30Z","actor":{"kind":"orchestrator","name":"core"},"type":"session_completed","payload":{"status":"completed"}}
`

func plantSession(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(root, "artifacts", ".bender", "sessions", "2026-04-16T14-03-22-zzz")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(plantedState), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(plantedEvents), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestSessions_List: T093.
func TestSessions_List(t *testing.T) {
	bin := buildBenderOnce(t)
	root := mkProject(t)
	plantSession(t, root)
	out, err := runBender(t, bin, root, "sessions", "list")
	if err != nil {
		t.Fatalf("sessions list: %v\n%s", err, out)
	}
	if !strings.Contains(out, "2026-04-16T14-03-22-zzz") || !strings.Contains(out, "/ghu") || !strings.Contains(out, "completed") {
		t.Fatalf("expected planted session in listing:\n%s", out)
	}
}

// TestSessions_Show_ByteIdentical: T094.
func TestSessions_Show_ByteIdentical(t *testing.T) {
	bin := buildBenderOnce(t)
	root := mkProject(t)
	plantSession(t, root)
	out, err := runBender(t, bin, root, "sessions", "show", "2026-04-16T14-03-22-zzz")
	if err != nil {
		t.Fatalf("sessions show: %v\n%s", err, out)
	}
	if out != plantedEvents {
		t.Fatalf("re-emission not byte-identical to events.jsonl:\nGOT:\n%s\nWANT:\n%s", out, plantedEvents)
	}
}

// TestSessions_Export_RoundTrip: T095 (SC-009).
func TestSessions_Export_RoundTrip(t *testing.T) {
	bin := buildBenderOnce(t)
	root := mkProject(t)
	plantSession(t, root)
	out, err := runBender(t, bin, root, "sessions", "export", "2026-04-16T14-03-22-zzz")
	if err != nil {
		t.Fatalf("sessions export: %v\n%s", err, out)
	}
	var doc struct {
		SchemaVersion int               `json:"schema_version"`
		SessionID     string            `json:"session_id"`
		Events        []json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("export not valid JSON: %v\n%s", err, out)
	}
	if doc.SessionID != "2026-04-16T14-03-22-zzz" || len(doc.Events) != 2 {
		t.Fatalf("doc shape wrong: %+v", doc)
	}
}

// TestSessions_Show_NotFound: contributes to T094.
func TestSessions_Show_NotFound(t *testing.T) {
	bin := buildBenderOnce(t)
	root := mkProject(t)
	out, err := runBender(t, bin, root, "sessions", "show", "no-such-session")
	if err == nil {
		t.Fatalf("expected non-zero exit for missing session\n%s", out)
	}
	if !strings.Contains(out, "no-such-session") {
		t.Fatalf("expected error to mention id\n%s", out)
	}
}

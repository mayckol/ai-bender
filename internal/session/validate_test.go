package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSession(t *testing.T, state, events string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(events), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestValidate_HappyPath(t *testing.T) {
	state := `{"schema_version":1,"session_id":"s1","command":"/plan","started_at":"2026-04-16T21:58:53Z","completed_at":"2026-04-16T21:58:55Z","status":"completed"}`
	events := strings.Join([]string{
		`{"schema_version":1,"session_id":"s1","timestamp":"2026-04-16T21:58:53Z","actor":{"kind":"user","name":"claude-code"},"type":"session_started","payload":{"command":"/plan","invoker":"u","working_dir":"/x"}}`,
		`{"schema_version":1,"session_id":"s1","timestamp":"2026-04-16T21:58:53Z","actor":{"kind":"stage","name":"plan"},"type":"stage_started","payload":{"stage":"plan","inputs":["a.md"]}}`,
		`{"schema_version":1,"session_id":"s1","timestamp":"2026-04-16T21:58:55Z","actor":{"kind":"stage","name":"plan"},"type":"stage_completed","payload":{"stage":"plan","outputs":["b.md"]}}`,
		`{"schema_version":1,"session_id":"s1","timestamp":"2026-04-16T21:58:55Z","actor":{"kind":"orchestrator","name":"plan"},"type":"session_completed","payload":{"status":"completed","duration_ms":2000}}`,
	}, "\n") + "\n"

	dir := writeSession(t, state, events)
	v, err := Validate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 0 {
		t.Fatalf("expected zero violations, got %v", v)
	}
}

func TestValidate_FlagsRealWorldGhuDrift(t *testing.T) {
	// Mirrors the exact drift observed in the brfiscalfaker /ghu session:
	// state.json missing schema_version; events missing invoker/working_dir, inputs, etc.
	state := `{"command":"/ghu","session_id":"s1","status":"completed","started_at":"2026-04-16T22:04:07Z","completed_at":"2026-04-16T22:04:28Z","files_changed":3,"findings_count":3}`
	events := strings.Join([]string{
		`{"schema_version":1,"session_id":"s1","timestamp":"2026-04-16T22:04:07Z","actor":{"kind":"orchestrator","name":"ghu"},"type":"session_started","payload":{"command":"/ghu"}}`,
		`{"schema_version":1,"session_id":"s1","timestamp":"2026-04-16T22:04:07Z","actor":{"kind":"orchestrator","name":"ghu"},"type":"stage_started","payload":{"stage":"ghu"}}`,
		`{"schema_version":1,"session_id":"s1","timestamp":"2026-04-16T22:04:28Z","actor":{"kind":"orchestrator","name":"ghu"},"type":"session_completed","payload":{"status":"completed"}}`,
	}, "\n") + "\n"

	dir := writeSession(t, state, events)
	v, err := Validate(dir)
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, x := range v {
		joined += x.String() + "\n"
	}
	wantSubstrings := []string{
		"state.json: schema_version=0",
		"invoker, working_dir",
		"inputs",
		"duration_ms",
		"no finding_reported events emitted",
	}
	for _, w := range wantSubstrings {
		if !strings.Contains(joined, w) {
			t.Errorf("want violation containing %q, got:\n%s", w, joined)
		}
	}
}

func TestValidate_SessionIDMismatch(t *testing.T) {
	state := `{"schema_version":1,"session_id":"s1","command":"/ghu","started_at":"2026-04-16T22:04:07Z","completed_at":"2026-04-16T22:04:08Z","status":"completed"}`
	events := strings.Join([]string{
		`{"schema_version":1,"session_id":"WRONG","timestamp":"2026-04-16T22:04:07Z","actor":{"kind":"orchestrator","name":"ghu"},"type":"session_started","payload":{"command":"/ghu","invoker":"u","working_dir":"/x"}}`,
		`{"schema_version":1,"session_id":"s1","timestamp":"2026-04-16T22:04:08Z","actor":{"kind":"orchestrator","name":"ghu"},"type":"session_completed","payload":{"status":"completed","duration_ms":1000}}`,
	}, "\n") + "\n"

	dir := writeSession(t, state, events)
	v, err := Validate(dir)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, x := range v {
		if strings.Contains(x.Message, "does not match state.session_id") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected session_id mismatch violation, got %v", v)
	}
}

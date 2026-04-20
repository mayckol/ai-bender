package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mayckol/ai-bender/internal/event"
)

// TestEventEmitParity_BashMirrorsBinary asserts that the Bash fallback writes a
// structurally equivalent JSON line to what the Go binary produces: same
// session_id, actor, type, and payload fields. Timestamps differ because both
// call-sites stamp at run time, so byte equality is not expected.
func TestEventEmitParity_BashMirrorsBinary(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not on PATH")
	}
	bashScript := repoRelPath(t, ".specify/scripts/bash/event-emit.sh")
	if _, err := os.Stat(bashScript); err != nil {
		t.Skipf("fallback script missing: %v", err)
	}

	sid := "parity-001"
	payload := `{"percent":25,"current_step":"scout","completed_nodes":2,"total_nodes":8}`

	// Leg A: binary (in-process, via event.Emit).
	rootA := t.TempDir()
	dirA := filepath.Join(rootA, sid)
	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatalf("mkdir A: %v", err)
	}
	var payloadMap map[string]any
	if err := json.Unmarshal([]byte(payload), &payloadMap); err != nil {
		t.Fatalf("payload parse: %v", err)
	}
	if err := event.Emit(event.EmitParams{
		SessionsRoot: rootA,
		SessionID:    sid,
		Type:         event.TypeOrchProgress,
		ActorKind:    event.ActorOrchestrator,
		ActorName:    "ghu",
		Payload:      payloadMap,
	}); err != nil {
		t.Fatalf("binary Emit: %v", err)
	}

	// Leg B: bash fallback.
	rootB := t.TempDir()
	dirB := filepath.Join(rootB, sid)
	if err := os.MkdirAll(dirB, 0o755); err != nil {
		t.Fatalf("mkdir B: %v", err)
	}
	cmd := exec.Command("bash", bashScript,
		"--sessions-root", rootB,
		"--session", sid,
		"--type", "orchestrator_progress",
		"--actor-kind", "orchestrator",
		"--actor-name", "ghu",
		"--payload", payload,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bash event-emit: %v\n%s", err, out)
	}

	lineA := readSingleLine(t, filepath.Join(dirA, "events.jsonl"))
	lineB := readSingleLine(t, filepath.Join(dirB, "events.jsonl"))

	evA, err := event.UnmarshalEvent(lineA)
	if err != nil {
		t.Fatalf("parse A: %v\n%s", err, lineA)
	}
	evB, err := event.UnmarshalEvent(lineB)
	if err != nil {
		t.Fatalf("parse B: %v\n%s", err, lineB)
	}

	if evA.SessionID != evB.SessionID {
		t.Errorf("session id mismatch: %s vs %s", evA.SessionID, evB.SessionID)
	}
	if evA.Actor != evB.Actor {
		t.Errorf("actor mismatch: %+v vs %+v", evA.Actor, evB.Actor)
	}
	if evA.Type != evB.Type {
		t.Errorf("type mismatch: %s vs %s", evA.Type, evB.Type)
	}
	if !equalPayloads(evA.Payload, evB.Payload) {
		t.Errorf("payload mismatch:\nA=%v\nB=%v", evA.Payload, evB.Payload)
	}
	if err := evA.Validate(); err != nil {
		t.Errorf("A invalid: %v", err)
	}
	if err := evB.Validate(); err != nil {
		t.Errorf("B invalid: %v", err)
	}
}

func readSingleLine(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	line := bytes.TrimRight(data, "\n")
	if bytes.Contains(line, []byte("\n")) {
		t.Fatalf("%s: expected single line, got multiple", path)
	}
	return line
}

func equalPayloads(a, b map[string]any) bool {
	aj, err1 := json.Marshal(canonicalize(a))
	bj, err2 := json.Marshal(canonicalize(b))
	if err1 != nil || err2 != nil {
		return false
	}
	return bytes.Equal(aj, bj)
}

// canonicalize rounds numeric values so that JSON's float64 decode doesn't
// cause spurious mismatches between legs (e.g. 25 vs 25.0 stored as float64).
func canonicalize(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		switch x := v.(type) {
		case float64:
			if x == float64(int64(x)) {
				out[k] = int64(x)
				continue
			}
			out[k] = x
		default:
			out[k] = v
		}
	}
	return out
}

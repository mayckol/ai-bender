package integration

import (
	"path/filepath"
	"testing"

	"github.com/mayckol/ai-bender/internal/event"
)

// TestScoutProgress_Monotonic asserts that once orchestrator_progress events
// are emitted through the new helper, their percent values are monotonically
// non-decreasing and reach 100 at session completion. The fixture simulates
// the "computed percent = 100 * completed_nodes / total_nodes" wiring the
// SKILL now documents (feature 007 US2).
func TestScoutProgress_Monotonic(t *testing.T) {
	root := t.TempDir()
	sid := "scout-progress-001"
	sessionDir := filepath.Join(root, sid)
	if err := makeDir(t, sessionDir); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	total := 10
	for i := 1; i <= total; i++ {
		percent := 100 * i / total
		err := event.Emit(event.EmitParams{
			SessionsRoot: root,
			SessionID:    sid,
			Type:         event.TypeOrchProgress,
			ActorKind:    event.ActorOrchestrator,
			ActorName:    "core",
			Payload: map[string]any{
				"percent":         percent,
				"current_step":    "node-" + itoa(i),
				"completed_nodes": i,
				"total_nodes":     total,
			},
		})
		if err != nil {
			t.Fatalf("emit #%d: %v", i, err)
		}
	}

	data := readFileBytes(t, filepath.Join(sessionDir, "events.jsonl"))
	lines := splitLines(data)
	if len(lines) != total {
		t.Fatalf("want %d lines, got %d", total, len(lines))
	}

	last := -1
	for idx, line := range lines {
		ev, err := event.UnmarshalEvent(line)
		if err != nil {
			t.Fatalf("line %d parse: %v", idx, err)
		}
		p := ev.Payload["percent"]
		pct, ok := asInt(p)
		if !ok {
			t.Fatalf("line %d percent not numeric: %v", idx, p)
		}
		if pct < last {
			t.Fatalf("non-monotonic at line %d: %d -> %d", idx, last, pct)
		}
		last = pct
	}
	if last != 100 {
		t.Fatalf("final percent %d, want 100", last)
	}
}

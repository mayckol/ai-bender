package clarification

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type cancellingPrompter struct{ called int }

func (c *cancellingPrompter) Ask(ctx context.Context, q Question) (Resolution, error) {
	c.called++
	return Resolution{QuestionID: q.ID, Kind: KindPendingNonInteractive, ResolvedAt: time.Now().UTC()}, ctx.Err()
}

func writeBatchInput(t *testing.T, dir string, b Batch) string {
	t.Helper()
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	p := filepath.Join(dir, "batch.json")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}
	return p
}

func TestRun_InterruptParksSessionAtAwaitingClarification(t *testing.T) {
	repo := t.TempDir()
	specDir := filepath.Join(repo, ".bender/artifacts/specs")
	captureDir := filepath.Join(repo, ".bender/artifacts/cry")
	planDir := filepath.Join(repo, ".bender/artifacts/plan")
	sessionDir := filepath.Join(repo, ".bender/sessions/sess1")
	for _, d := range []string{specDir, captureDir, planDir, sessionDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	specPath := filepath.Join(specDir, "foo-2026.md")
	if err := os.WriteFile(specPath, []byte("# spec\n- FR-001: x [NEEDS CLARIFICATION: y]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	capturePath := filepath.Join(captureDir, "foo-2026.md")
	if err := os.WriteFile(capturePath, []byte("# cap\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(sessionDir, "state.json")
	if err := os.WriteFile(statePath, []byte(`{"schema_version":1,"session_id":"sess1","command":"/plan","started_at":"2026-04-19T10:00:00Z","status":"running"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	batch := mkBatchN(1)
	inputPath := writeBatchInput(t, repo, batch)

	outPath := filepath.Join(planDir, "clarifications-2026-04-19T10-00-00-000.md")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled — Prompter sees ctx.Err() on first call

	opts := Options{
		FromSpec:    specPath,
		FromCapture: capturePath,
		Timestamp:   "2026-04-19T10-00-00-000",
		OutPath:     outPath,
		SessionDir:  sessionDir,
		InputPath:   inputPath,
		Prompter:    &cancellingPrompter{},
	}
	err := Run(ctx, opts)
	if err == nil {
		t.Fatal("expected ctx-cancellation error")
	}

	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("clarifications artifact not persisted: %v", err)
	}

	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	state := string(stateBytes)
	if !strings.Contains(state, `"status": "awaiting_clarification"`) {
		t.Fatalf("expected awaiting_clarification status:\n%s", state)
	}
	if !strings.Contains(state, `"clarifications_artifact"`) {
		t.Fatalf("expected clarifications_artifact field:\n%s", state)
	}
}

package integration

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mayckol/ai-bender/internal/event"
	"github.com/mayckol/ai-bender/internal/server"
	"github.com/mayckol/ai-bender/internal/worktree"
	"github.com/mayckol/ai-bender/tests/integration/helpers"
)

// TestFlowView_WorktreeEmitsStreamLive asserts that a /ghu session running
// inside a worktree path emits events that land on the session SSE stream
// within a short bounded delay (feature 007, story P1). Events are written
// via event.Emit with an absolute sessions-root, mirroring what the skill
// now does when cwd is the worktree instead of the main repo.
func TestFlowView_WorktreeEmitsStreamLive(t *testing.T) {
	repo := helpers.NewSeededRepo(t)
	out, err := worktree.Create(context.Background(), worktree.CreateInput{
		RepoRoot:   repo,
		SessionID:  "flow-001",
		Runner:     &worktree.ExecRunner{},
		WorkflowID: "wf-flow-001",
	})
	if err != nil {
		t.Fatalf("worktree create: %v", err)
	}

	h, err := server.New(server.Config{ProjectRoot: repo})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	ts := httptest.NewServer(h)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/sessions/flow-001/stream", nil)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("SSE connect: %v", err)
	}
	defer resp.Body.Close()

	events := make(chan string, 16)
	go func() {
		defer close(events)
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data:") {
				events <- strings.TrimPrefix(line, "data:")
			}
		}
	}()

	// Wait for the initial snapshot event before emitting so the tail
	// subscription is live.
	if !waitFor(events, 2*time.Second, func(s string) bool { return strings.Contains(s, "\"events\"") }) {
		t.Fatal("initial snapshot not received")
	}

	sessionsRoot := filepath.Join(repo, ".bender", "sessions")
	for i := 0; i < 3; i++ {
		if err := event.Emit(event.EmitParams{
			SessionsRoot: sessionsRoot,
			SessionID:    "flow-001",
			Type:         event.TypeOrchProgress,
			ActorKind:    event.ActorOrchestrator,
			ActorName:    "core",
			Payload: map[string]any{
				"percent":         25 + i*25,
				"current_step":    "scout",
				"completed_nodes": i + 1,
				"total_nodes":     4,
			},
		}); err != nil {
			t.Fatalf("emit #%d: %v", i, err)
		}
	}

	seen := 0
	deadline := time.After(3 * time.Second)
	for seen < 3 {
		select {
		case data, ok := <-events:
			if !ok {
				t.Fatalf("SSE closed; saw %d of 3 events", seen)
			}
			// Tolerate occasional state-patch lines by filtering for progress events.
			if strings.Contains(data, "orchestrator_progress") {
				var parsed map[string]any
				if err := json.Unmarshal([]byte(data), &parsed); err != nil {
					continue
				}
				seen++
			}
		case <-deadline:
			t.Fatalf("timed out: saw %d of 3 progress events", seen)
		}
	}

	_ = out
}

func waitFor(ch <-chan string, timeout time.Duration, pred func(string) bool) bool {
	deadline := time.After(timeout)
	for {
		select {
		case s, ok := <-ch:
			if !ok {
				return false
			}
			if pred(s) {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

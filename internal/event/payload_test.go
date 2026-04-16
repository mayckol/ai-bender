package event

import (
	"strings"
	"testing"
	"time"
)

func okEvent(t Type, payload map[string]any) *Event {
	return &Event{
		SchemaVersion: SchemaVersion,
		SessionID:     "s1",
		Timestamp:     time.Now(),
		Actor:         Actor{Kind: ActorOrchestrator, Name: "ghu"},
		Type:          t,
		Payload:       payload,
	}
}

func TestValidatePayload_SessionStarted_RequiresInvokerAndWorkingDir(t *testing.T) {
	e := okEvent(TypeSessionStarted, map[string]any{"command": "/ghu"})
	err := e.ValidatePayload()
	if err == nil {
		t.Fatal("expected error for missing invoker + working_dir")
	}
	for _, want := range []string{"invoker", "working_dir"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q: %v", want, err)
		}
	}
}

func TestValidatePayload_StageCompleted_RequiresOutputs(t *testing.T) {
	e := okEvent(TypeStageCompleted, map[string]any{"stage": "ghu"})
	if err := e.ValidatePayload(); err == nil || !strings.Contains(err.Error(), "outputs") {
		t.Fatalf("want outputs-missing error, got %v", err)
	}
}

func TestValidatePayload_AgentCompleted_RequiresTaskIdsAndDuration(t *testing.T) {
	e := okEvent(TypeAgentCompleted, map[string]any{"agent": "crafter"})
	err := e.ValidatePayload()
	if err == nil {
		t.Fatal("expected missing task_ids + duration_ms")
	}
	for _, want := range []string{"task_ids", "duration_ms"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q: %v", want, err)
		}
	}
}

func TestValidatePayload_FileChanged_RequiresAgent(t *testing.T) {
	e := okEvent(TypeFileChanged, map[string]any{
		"path":          "pkg/foo.go",
		"lines_added":   1,
		"lines_removed": 0,
	})
	if err := e.ValidatePayload(); err == nil || !strings.Contains(err.Error(), "agent") {
		t.Fatalf("want agent-missing error, got %v", err)
	}
}

func TestValidatePayload_HappyPath(t *testing.T) {
	e := okEvent(TypeSessionCompleted, map[string]any{
		"status":      "completed",
		"duration_ms": 1234,
	})
	if err := e.ValidatePayload(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePayload_NumericZeroIsAccepted(t *testing.T) {
	e := okEvent(TypeFileChanged, map[string]any{
		"path":          "pkg/foo.go",
		"lines_added":   0,
		"lines_removed": 0,
		"agent":         "crafter",
	})
	if err := e.ValidatePayload(); err != nil {
		t.Fatalf("lines_added=0 is valid, got %v", err)
	}
}

func TestValidatePayload_EmptyStringIsMissing(t *testing.T) {
	e := okEvent(TypeSkillInvoked, map[string]any{"skill": "bg-crafter-implement", "agent": ""})
	if err := e.ValidatePayload(); err == nil || !strings.Contains(err.Error(), "agent") {
		t.Fatalf("want agent-missing error for empty string, got %v", err)
	}
}

func TestValidatePayload_UnknownTypeNoOp(t *testing.T) {
	e := okEvent(Type("something_new"), nil)
	if err := e.ValidatePayload(); err != nil {
		t.Fatalf("unknown types should be tolerated by payload check, got %v", err)
	}
}

func TestRequiredPayloadFields_IsCopy(t *testing.T) {
	got := RequiredPayloadFields(TypeSessionCompleted)
	got[0] = "mutated"
	again := RequiredPayloadFields(TypeSessionCompleted)
	if again[0] == "mutated" {
		t.Fatal("RequiredPayloadFields must return a copy, not the internal slice")
	}
}

package event

import (
	"errors"
	"testing"
	"time"
)

func validEvent() *Event {
	return &Event{
		SchemaVersion: SchemaVersion,
		SessionID:     "2026-04-16T14-03-22-a3f",
		Timestamp:     time.Date(2026, 4, 16, 14, 3, 22, 0, time.UTC),
		Actor:         Actor{Kind: ActorAgent, Name: "crafter"},
		Type:          TypeAgentProgress,
		Payload:       map[string]any{"percent": 42},
	}
}

func TestEvent_Validate_AcceptsValid(t *testing.T) {
	if err := validEvent().Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestEvent_Validate_RejectsWrongSchemaVersion(t *testing.T) {
	e := validEvent()
	e.SchemaVersion = 99
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for wrong schema version")
	}
}

func TestEvent_Validate_RequiresSessionID(t *testing.T) {
	e := validEvent()
	e.SessionID = ""
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for missing session_id")
	}
}

func TestEvent_Validate_RequiresTimestamp(t *testing.T) {
	e := validEvent()
	e.Timestamp = time.Time{}
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for zero timestamp")
	}
}

func TestEvent_Validate_RejectsUnknownActorKind(t *testing.T) {
	e := validEvent()
	e.Actor.Kind = "alien"
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for unknown actor kind")
	}
}

func TestEvent_Validate_RejectsUnknownType(t *testing.T) {
	e := validEvent()
	e.Type = "made_up_event"
	err := e.Validate()
	if !errors.Is(err, ErrUnknownType) {
		t.Fatalf("expected ErrUnknownType, got %v", err)
	}
}

func TestEvent_MarshalUnmarshalRoundTrip(t *testing.T) {
	e := validEvent()
	line, err := e.MarshalJSONLine()
	if err != nil {
		t.Fatalf("MarshalJSONLine: %v", err)
	}
	if line[len(line)-1] != '\n' {
		t.Fatal("MarshalJSONLine must terminate with newline for events.jsonl")
	}
	got, err := UnmarshalEvent(line[:len(line)-1])
	if err != nil {
		t.Fatalf("UnmarshalEvent: %v", err)
	}
	if got.Type != e.Type || got.Actor != e.Actor || got.SessionID != e.SessionID {
		t.Fatalf("round trip mismatch: %+v vs %+v", got, e)
	}
}

func TestKnownTypes_CoversAllEventConstants(t *testing.T) {
	// Sanity check: every constant we expect is in the KnownTypes() set.
	want := []Type{
		TypeSessionStarted, TypeStageStarted, TypeStageCompleted, TypeStageFailed,
		TypeOrchDecision, TypeAgentStarted, TypeAgentCompleted, TypeAgentFailed,
		TypeAgentBlocked, TypeAgentProgress, TypeAgentLog,
		TypeSkillInvoked, TypeSkillCompleted, TypeSkillFailed,
		TypeArtifactWritten, TypeFileChanged, TypeFindingReported, TypeSessionCompleted,
	}
	known := map[Type]bool{}
	for _, t := range KnownTypes() {
		known[t] = true
	}
	for _, w := range want {
		if !known[w] {
			t.Errorf("KnownTypes missing %q", w)
		}
	}
}

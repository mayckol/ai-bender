package event

import (
	"testing"
	"time"
)

func ev(t Type, actor Actor, payload map[string]any) *Event {
	return &Event{
		SchemaVersion: SchemaVersion,
		SessionID:     "s1",
		Timestamp:     time.Now(),
		Actor:         actor,
		Type:          t,
		Payload:       payload,
	}
}

func TestResponsibleAgent_PrefersPayloadAgent(t *testing.T) {
	e := ev(TypeSkillInvoked, Actor{Kind: ActorAgent, Name: "tester"}, map[string]any{"agent": "crafter"})
	if got := ResponsibleAgent(e); got != "crafter" {
		t.Fatalf("got %q, want crafter", got)
	}
}

func TestResponsibleAgent_DispatchedAgentForOrchestrator(t *testing.T) {
	e := ev(TypeOrchDecision, Actor{Kind: ActorOrchestrator, Name: "ghu"}, map[string]any{
		"decision_type":    "agent_assignment",
		"dispatched_agent": "crafter",
	})
	if got := ResponsibleAgent(e); got != "crafter" {
		t.Fatalf("got %q, want crafter", got)
	}
}

func TestResponsibleAgent_AgentActorFallback(t *testing.T) {
	e := ev(TypeAgentStarted, Actor{Kind: ActorAgent, Name: "reviewer"}, nil)
	if got := ResponsibleAgent(e); got != "reviewer" {
		t.Fatalf("got %q, want reviewer", got)
	}
}

func TestResponsibleAgent_OrchestratorBecomesMain(t *testing.T) {
	e := ev(TypeOrchDecision, Actor{Kind: ActorOrchestrator, Name: "ghu"}, map[string]any{
		"decision_type": "task_decomposition",
	})
	if got := ResponsibleAgent(e); got != "main" {
		t.Fatalf("got %q, want main", got)
	}
}

func TestResponsibleAgent_StageBecomesMain(t *testing.T) {
	e := ev(TypeStageStarted, Actor{Kind: ActorStage, Name: "plan"}, map[string]any{"stage": "plan"})
	if got := ResponsibleAgent(e); got != "main" {
		t.Fatalf("got %q, want main", got)
	}
}

func TestResponsibleAgent_UserBecomesMain(t *testing.T) {
	e := ev(TypeSessionStarted, Actor{Kind: ActorUser, Name: "claude-code"}, map[string]any{})
	if got := ResponsibleAgent(e); got != "main" {
		t.Fatalf("got %q, want main", got)
	}
}

func TestSkillName_ExtractsOnlyForSkillEvents(t *testing.T) {
	inv := ev(TypeSkillInvoked, Actor{Kind: ActorAgent, Name: "c"}, map[string]any{"skill": "bg-crafter-implement"})
	if got := SkillName(inv); got != "bg-crafter-implement" {
		t.Fatalf("got %q, want bg-crafter-implement", got)
	}
	other := ev(TypeFileChanged, Actor{Kind: ActorAgent, Name: "c"}, map[string]any{"skill": "nope"})
	if got := SkillName(other); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

// Package event defines the v1 event schema used in .bender/sessions/<id>/events.jsonl.
// Events are written by Claude Code (or any equivalent client) when executing a slash command;
// the bender binary reads them via `bender sessions show/export`.
package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// SchemaVersion is the current event schema version. Backwards-compatible field additions
// are allowed within v1; breaking changes require bumping this constant.
const SchemaVersion = 1

// ActorKind enumerates who emitted an event.
type ActorKind string

const (
	ActorOrchestrator ActorKind = "orchestrator"
	ActorAgent        ActorKind = "agent"
	ActorStage        ActorKind = "stage"
	ActorSink         ActorKind = "sink"
	ActorUser         ActorKind = "user"
)

func KnownActorKinds() []ActorKind {
	return []ActorKind{ActorOrchestrator, ActorAgent, ActorStage, ActorSink, ActorUser}
}

// Actor identifies the source of an event.
type Actor struct {
	Kind ActorKind `json:"kind"`
	Name string    `json:"name"`
}

// Type enumerates the v1 event types.
type Type string

const (
	TypeSessionStarted   Type = "session_started"
	TypeStageStarted     Type = "stage_started"
	TypeStageCompleted   Type = "stage_completed"
	TypeStageFailed      Type = "stage_failed"
	TypeOrchDecision     Type = "orchestrator_decision"
	TypeOrchProgress     Type = "orchestrator_progress"
	TypeAgentStarted     Type = "agent_started"
	TypeAgentCompleted   Type = "agent_completed"
	TypeAgentFailed      Type = "agent_failed"
	TypeAgentBlocked     Type = "agent_blocked"
	TypeAgentProgress    Type = "agent_progress"
	TypeAgentLog         Type = "agent_log"
	TypeSkillInvoked     Type = "skill_invoked"
	TypeSkillCompleted   Type = "skill_completed"
	TypeSkillFailed      Type = "skill_failed"
	TypeArtifactWritten  Type = "artifact_written"
	TypeFileChanged      Type = "file_changed"
	TypeFindingReported  Type = "finding_reported"
	TypeSessionCompleted Type = "session_completed"

	// Worktree lifecycle kinds added for feature 004-worktree-flow.
	TypeWorktreeCreated Type = "worktree_created"
	TypeWorktreeRemoved Type = "worktree_removed"
	TypeWorktreeMissing Type = "worktree_missing"

	// Optional pull-request kinds added for feature 004-worktree-flow.
	TypePROpened        Type = "pr_opened"
	TypePRUpdateRefused Type = "pr_update_refused"
)

// KnownTypes returns every event type defined in v1.
func KnownTypes() []Type {
	return []Type{
		TypeSessionStarted,
		TypeStageStarted, TypeStageCompleted, TypeStageFailed,
		TypeOrchDecision, TypeOrchProgress,
		TypeAgentStarted, TypeAgentCompleted, TypeAgentFailed, TypeAgentBlocked,
		TypeAgentProgress, TypeAgentLog,
		TypeSkillInvoked, TypeSkillCompleted, TypeSkillFailed,
		TypeArtifactWritten, TypeFileChanged,
		TypeFindingReported,
		TypeSessionCompleted,
		TypeWorktreeCreated, TypeWorktreeRemoved, TypeWorktreeMissing,
		TypePROpened, TypePRUpdateRefused,
	}
}

// Event is one record in events.jsonl.
type Event struct {
	SchemaVersion int            `json:"schema_version"`
	SessionID     string         `json:"session_id"`
	Timestamp     time.Time      `json:"timestamp"`
	Actor         Actor          `json:"actor"`
	Type          Type           `json:"type"`
	Payload       map[string]any `json:"payload,omitempty"`
}

// ErrUnknownType is returned when an event has a type that v1 does not recognise.
var ErrUnknownType = errors.New("unknown event type")

// Validate checks the structural invariants required by `contracts/event-schema.md`.
func (e *Event) Validate() error {
	if e.SchemaVersion != SchemaVersion {
		return fmt.Errorf("event: schema_version=%d (want %d)", e.SchemaVersion, SchemaVersion)
	}
	if e.SessionID == "" {
		return errors.New("event: session_id is required")
	}
	if e.Timestamp.IsZero() {
		return errors.New("event: timestamp is required")
	}
	if e.Actor.Kind == "" || e.Actor.Name == "" {
		return errors.New("event: actor.kind and actor.name are required")
	}
	if !isKnownActorKind(e.Actor.Kind) {
		return fmt.Errorf("event: unknown actor.kind %q", e.Actor.Kind)
	}
	if !isKnownType(e.Type) {
		return fmt.Errorf("%w: %q", ErrUnknownType, e.Type)
	}
	return nil
}

func isKnownActorKind(k ActorKind) bool {
	for _, candidate := range KnownActorKinds() {
		if k == candidate {
			return true
		}
	}
	return false
}

func isKnownType(t Type) bool {
	for _, candidate := range KnownTypes() {
		if t == candidate {
			return true
		}
	}
	return false
}

// MarshalJSONLine returns a single newline-terminated JSON line suitable for events.jsonl.
func (e *Event) MarshalJSONLine() ([]byte, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// UnmarshalEvent parses a single JSON line back into an Event.
func UnmarshalEvent(line []byte) (*Event, error) {
	var e Event
	if err := json.Unmarshal(line, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

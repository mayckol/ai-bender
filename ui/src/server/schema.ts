// Mirrors internal/event/event.go and internal/session/state.go.
// Kept hand-written and small — 20 lines of drift-checkable types beats a
// generated file for v1 scope. If this drifts, `bender sessions validate`
// catches it on the CLI side.

export const SCHEMA_VERSION = 1;

export type ActorKind =
  | 'orchestrator'
  | 'agent'
  | 'stage'
  | 'sink'
  | 'user';

export type EventType =
  | 'session_started'
  | 'stage_started'
  | 'stage_completed'
  | 'stage_failed'
  | 'orchestrator_decision'
  | 'agent_started'
  | 'agent_completed'
  | 'agent_failed'
  | 'agent_blocked'
  | 'agent_progress'
  | 'agent_log'
  | 'skill_invoked'
  | 'skill_completed'
  | 'skill_failed'
  | 'artifact_written'
  | 'file_changed'
  | 'finding_reported'
  | 'session_completed';

export interface Actor {
  kind: ActorKind;
  name: string;
}

export interface BenderEvent {
  schema_version: number;
  session_id: string;
  timestamp: string;
  actor: Actor;
  type: EventType;
  payload?: Record<string, unknown>;
}

export type SessionStatus = 'running' | 'completed' | 'failed';

export interface SessionState {
  schema_version?: number;
  session_id: string;
  command: string;
  started_at: string;
  completed_at?: string;
  status: SessionStatus;
  source_artifacts?: string[];
  skills_invoked?: string[];
  files_changed?: number;
  findings_count?: number;
  // Optional extras some /ghu runs add. Not in the v1 contract; kept
  // permissive so the viewer doesn't reject pre-existing sessions.
  blockers_count?: number;
  report_path?: string;
}

export interface SessionSummary {
  id: string;
  state: SessionState;
  duration_ms: number;
}

export interface SessionExport {
  state: SessionState;
  events: BenderEvent[];
}

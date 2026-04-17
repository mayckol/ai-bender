import { readdir, readFile } from 'node:fs/promises';
import { join } from 'node:path';

import type {
  BenderEvent,
  SessionExport,
  SessionState,
  SessionSummary,
} from './schema.ts';

export function sessionsRoot(projectRoot: string): string {
  return join(projectRoot, '.bender', 'sessions');
}

export function sessionDir(projectRoot: string, id: string): string {
  return join(sessionsRoot(projectRoot), id);
}

export async function listSessions(projectRoot: string): Promise<SessionSummary[]> {
  const root = sessionsRoot(projectRoot);
  let entries;
  try {
    entries = await readdir(root, { withFileTypes: true });
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === 'ENOENT') return [];
    throw err;
  }

  const out: SessionSummary[] = [];
  for (const e of entries) {
    if (!e.isDirectory()) continue;
    const id = e.name;
    const dir = join(root, id);
    try {
      const state = await readState(dir);
      const events = await readEvents(dir);
      const duration_ms = computeDuration(state.started_at, events);
      const { agents, skills } = summarizeEvents(events);
      out.push({ id, state, duration_ms, agents, skills });
    } catch {
      // Skip malformed session dirs; they surface via `bender sessions validate`.
    }
  }
  out.sort((a, b) => a.id.localeCompare(b.id));
  return out;
}

const SKILL_EVENT_TYPES = new Set(['skill_invoked', 'skill_completed', 'skill_failed']);

export function summarizeEvents(events: BenderEvent[]): { agents: string[]; skills: string[] } {
  const agents = new Set<string>();
  const skills = new Set<string>();
  for (const ev of events) {
    agents.add(responsibleAgentForSummary(ev));
    if (SKILL_EVENT_TYPES.has(ev.type)) {
      const skill = (ev.payload as Record<string, unknown> | undefined)?.skill;
      if (typeof skill === 'string' && skill) skills.add(skill);
    }
  }
  return {
    agents: [...agents].sort(),
    skills: [...skills].sort(),
  };
}

// Duplicated from client/lib/agents.ts so the server module has no client deps.
// Keep in sync with `ResponsibleAgent` in internal/event/agent.go.
function responsibleAgentForSummary(ev: BenderEvent): string {
  const p = (ev.payload ?? {}) as Record<string, unknown>;
  if (typeof p.agent === 'string' && p.agent) return p.agent;
  if (typeof p.dispatched_agent === 'string' && p.dispatched_agent) return p.dispatched_agent;
  if (ev.actor.kind === 'agent' && ev.actor.name) return ev.actor.name;
  return 'main';
}

export async function readState(dir: string): Promise<SessionState> {
  const raw = await readFile(join(dir, 'state.json'), 'utf8');
  return JSON.parse(raw) as SessionState;
}

export async function readEvents(dir: string): Promise<BenderEvent[]> {
  try {
    const raw = await readFile(join(dir, 'events.jsonl'), 'utf8');
    return parseEventsJSONL(raw);
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === 'ENOENT') return [];
    throw err;
  }
}

export function parseEventsJSONL(raw: string): BenderEvent[] {
  const out: BenderEvent[] = [];
  for (const line of raw.split('\n')) {
    if (!line.trim()) continue;
    try {
      out.push(JSON.parse(line) as BenderEvent);
    } catch {
      // Skip malformed lines; keep everything else.
    }
  }
  return out;
}

export async function exportSession(dir: string): Promise<SessionExport> {
  const [state, events] = await Promise.all([readState(dir), readEvents(dir)]);
  return { state, events };
}

export function computeDuration(startedAt: string, events: BenderEvent[]): number {
  if (!events.length || !startedAt) return 0;
  const last = events[events.length - 1];
  const start = Date.parse(startedAt);
  const end = Date.parse(last.timestamp);
  if (!Number.isFinite(start) || !Number.isFinite(end)) return 0;
  return Math.max(0, end - start);
}

// Convention from internal/embed/defaults/claude/skills/ghu/SKILL.md:
// .bender/artifacts/ghu/run-<timestamp>-report.md where <timestamp> is the
// session id stripped of its trailing `-xxx` suffix. Strip by matching the
// timestamp head so we tolerate variations.
const SESSION_TS_HEAD = /^(\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2})/;

export function reportPath(projectRoot: string, sessionId: string): string {
  const m = sessionId.match(SESSION_TS_HEAD);
  const ts = m ? m[1] : sessionId;
  return join(projectRoot, '.bender', 'artifacts', 'ghu', `run-${ts}-report.md`);
}

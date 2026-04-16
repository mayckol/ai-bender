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
      out.push({ id, state, duration_ms });
    } catch {
      // Skip malformed session dirs; they surface via `bender sessions validate`.
    }
  }
  out.sort((a, b) => a.id.localeCompare(b.id));
  return out;
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

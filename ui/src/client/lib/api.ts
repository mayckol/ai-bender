import type {
  BenderEvent,
  SessionExport,
  SessionState,
  SessionSummary,
} from '../../server/schema.ts';

export type { BenderEvent, SessionExport, SessionState, SessionSummary };

export async function fetchSessions(): Promise<SessionSummary[]> {
  const r = await fetch('/api/sessions');
  if (!r.ok) throw new Error(`GET /api/sessions → ${r.status}`);
  return r.json() as Promise<SessionSummary[]>;
}

export async function fetchSession(id: string): Promise<SessionExport> {
  const r = await fetch(`/api/sessions/${encodeURIComponent(id)}`);
  if (!r.ok) throw new Error(`GET /api/sessions/${id} → ${r.status}`);
  return r.json() as Promise<SessionExport>;
}

export function reportUrl(id: string): string {
  return `/api/sessions/${encodeURIComponent(id)}/report`;
}

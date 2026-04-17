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

export async function deleteSession(id: string): Promise<void> {
  const r = await fetch(`/api/sessions/${encodeURIComponent(id)}`, { method: 'DELETE' });
  if (!r.ok) throw new Error(`DELETE /api/sessions/${id} → ${r.status}`);
}

export async function deleteAllSessions(): Promise<number> {
  const r = await fetch('/api/sessions?all=true', { method: 'DELETE' });
  if (!r.ok) throw new Error(`DELETE /api/sessions?all=true → ${r.status}`);
  const body = await r.json().catch(() => ({ removed: 0 }));
  return typeof body.removed === 'number' ? body.removed : 0;
}

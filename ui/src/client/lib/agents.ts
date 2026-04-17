import type { BenderEvent } from './api.ts';

/**
 * Canonical agent name for an event. Matches the Go helper in
 * internal/event/agent.go so the server list and the client timeline always
 * agree on who owns an event.
 *
 * Mental model:
 * - Inline stages (/cry, /plan, /tdd, /bender-bootstrap, /ghu --inline) all
 *   happen in the "main" conversation — there are no worker subagents, so we
 *   attribute everything to `main` instead of the raw actor name.
 * - /ghu --bg delegates to specific workers (crafter, tester, reviewer, …).
 *   Those events carry the worker name in payload.agent or actor.name.
 *
 * Precedence:
 *  1. payload.agent              — explicit on agent-produced events
 *  2. payload.dispatched_agent   — orchestrator_decision targeting an agent
 *  3. actor.name if actor.kind === 'agent'
 *  4. 'main' for everything else (orchestrator, stage, user, sink)
 */
export const MAIN_AGENT = 'main';

export function responsibleAgent(ev: BenderEvent): string {
  const p = (ev.payload ?? {}) as Record<string, unknown>;
  if (typeof p.agent === 'string' && p.agent) return p.agent;
  if (typeof p.dispatched_agent === 'string' && p.dispatched_agent) return p.dispatched_agent;
  if (ev.actor.kind === 'agent' && ev.actor.name) return ev.actor.name;
  return MAIN_AGENT;
}

/**
 * Deterministic HSL color for an agent name. Stable across reloads because
 * it hashes the name — the same agent always renders in the same color.
 */
export function agentColor(name: string): string {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = (hash * 31 + name.charCodeAt(i)) | 0;
  }
  const hue = ((hash % 360) + 360) % 360;
  return `hsl(${hue}deg 55% 60%)`;
}

export function distinctAgents(events: BenderEvent[]): string[] {
  const seen = new Set<string>();
  for (const ev of events) seen.add(responsibleAgent(ev));
  return [...seen].sort();
}

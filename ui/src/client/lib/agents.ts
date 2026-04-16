import type { BenderEvent } from './api.ts';

/**
 * Given an event, determine which agent is responsible for it so the UI can
 * thread events by agent regardless of whether the actor is the agent itself
 * (agent_* events) or the orchestrator dispatching work to an agent
 * (orchestrator_decision with dispatched_agent).
 *
 * Preference order:
 *  1. payload.agent                — explicitly declared (required on most v1 events)
 *  2. payload.dispatched_agent     — orchestrator_decision targeting an agent
 *  3. actor.name when actor.kind is 'agent' — agent speaking for itself
 *  4. 'orchestrator' / 'stage'     — fallback to actor kind so timelines still render
 */
export function responsibleAgent(ev: BenderEvent): string {
  const p = (ev.payload ?? {}) as Record<string, unknown>;
  if (typeof p.agent === 'string' && p.agent) return p.agent;
  if (typeof p.dispatched_agent === 'string' && p.dispatched_agent) return p.dispatched_agent;
  if (ev.actor.kind === 'agent' && ev.actor.name) return ev.actor.name;
  return ev.actor.name || ev.actor.kind;
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

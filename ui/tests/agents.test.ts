import { describe, expect, test } from 'bun:test';

import type { BenderEvent } from '../src/server/schema.ts';
import { agentColor, distinctAgents, responsibleAgent } from '../src/client/lib/agents.ts';

function ev(overrides: Partial<BenderEvent>): BenderEvent {
  return {
    schema_version: 1,
    session_id: 's1',
    timestamp: '2026-04-16T22:04:07Z',
    actor: { kind: 'orchestrator', name: 'ghu' },
    type: 'skill_invoked',
    payload: {},
    ...overrides,
  };
}

describe('responsibleAgent', () => {
  test('prefers payload.agent when present', () => {
    const e = ev({ payload: { agent: 'crafter' }, actor: { kind: 'agent', name: 'tester' } });
    expect(responsibleAgent(e)).toBe('crafter');
  });

  test('uses payload.dispatched_agent for orchestrator decisions', () => {
    const e = ev({
      type: 'orchestrator_decision',
      payload: { decision_type: 'agent_assignment', dispatched_agent: 'crafter' },
    });
    expect(responsibleAgent(e)).toBe('crafter');
  });

  test('falls back to actor.name when actor is an agent', () => {
    const e = ev({ payload: {}, actor: { kind: 'agent', name: 'reviewer' } });
    expect(responsibleAgent(e)).toBe('reviewer');
  });

  test('falls back to actor name for non-agent actors', () => {
    const e = ev({ payload: {}, actor: { kind: 'orchestrator', name: 'ghu' } });
    expect(responsibleAgent(e)).toBe('ghu');
  });
});

describe('agentColor', () => {
  test('returns a stable HSL color for the same input', () => {
    expect(agentColor('crafter')).toBe(agentColor('crafter'));
  });

  test('produces distinct colors for distinct agent names in practice', () => {
    const colors = new Set(['crafter', 'tester', 'reviewer', 'linter', 'ghu'].map(agentColor));
    expect(colors.size).toBeGreaterThan(3);
  });
});

describe('distinctAgents', () => {
  test('returns the sorted set of responsible agents', () => {
    const events = [
      ev({ payload: { agent: 'crafter' } }),
      ev({ payload: { agent: 'tester' } }),
      ev({ payload: { agent: 'crafter' } }),
      ev({ actor: { kind: 'orchestrator', name: 'ghu' }, payload: {} }),
    ];
    expect(distinctAgents(events)).toEqual(['crafter', 'ghu', 'tester']);
  });
});

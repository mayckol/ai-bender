import { describe, expect, test } from 'bun:test';

import type { SessionSummary } from '../src/server/schema.ts';
import { pickLiveWorkflowId } from '../src/client/lib/project-flow.ts';

function sum(partial: Partial<SessionSummary> & { id: string; command: string; status?: string; workflowId?: string }): SessionSummary {
  return {
    id: partial.id,
    duration_ms: 0,
    agents: [],
    skills: [],
    state: {
      session_id: partial.id,
      command: partial.command,
      started_at: '2026-04-20T12:00:00Z',
      status: (partial.status ?? 'completed') as SessionSummary['state']['status'],
      workflow_id: partial.workflowId,
    },
  };
}

describe('pickLiveWorkflowId', () => {
  test('returns null when no session carries a workflow_id', () => {
    const sessions: SessionSummary[] = [
      sum({ id: 's1', command: '/ghu' }),
      sum({ id: 's2', command: '/tdd' }),
    ];
    expect(pickLiveWorkflowId(sessions)).toBeNull();
  });

  test('prefers running session with a workflow_id', () => {
    const sessions: SessionSummary[] = [
      sum({ id: 's1-old', command: '/tdd', workflowId: 'wf-a', status: 'completed' }),
      sum({ id: 's2-run', command: '/ghu', workflowId: 'wf-b', status: 'running' }),
    ];
    expect(pickLiveWorkflowId(sessions)).toBe('wf-b');
  });

  test('falls back to newest workflow-linked session when none are running', () => {
    const sessions: SessionSummary[] = [
      sum({ id: '2026-04-20T12-00-00-aaa', command: '/tdd', workflowId: 'wf-a' }),
      sum({ id: '2026-04-20T13-00-00-bbb', command: '/ghu', workflowId: 'wf-b' }),
    ];
    expect(pickLiveWorkflowId(sessions)).toBe('wf-b');
  });
});

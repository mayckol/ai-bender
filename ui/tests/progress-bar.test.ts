import { describe, expect, test } from 'bun:test';
import { render } from 'preact-render-to-string';
import { h } from 'preact';

import type { BenderEvent } from '../src/server/schema.ts';
import { ProgressBar } from '../src/client/components/ProgressBar.tsx';

function ev(partial: Partial<BenderEvent> & { type: string; payload: Record<string, unknown> }): BenderEvent {
  const base = {
    schema_version: 1,
    session_id: 's1',
    timestamp: '2026-04-20T12:00:00Z',
    actor: { kind: 'orchestrator', name: 'core' },
  };
  return { ...base, ...partial } as BenderEvent;
}

describe('ProgressBar (feature 007)', () => {
  test('renders session percent from orchestrator_progress monotonically', () => {
    const events = [
      ev({ type: 'orchestrator_progress', payload: { percent: 25, current_step: 'scout' } }),
      ev({ type: 'orchestrator_progress', payload: { percent: 10, current_step: 'regress' } }), // regression ignored
      ev({ type: 'orchestrator_progress', payload: { percent: 75, current_step: 'crafter' } }),
    ];
    const html = render(h(ProgressBar, { events, frozen: false }));
    expect(html).toContain('>75<');
    expect(html).not.toContain('>10<');
  });

  test('shows completed/total nodes in tooltip when present', () => {
    const events = [
      ev({
        type: 'orchestrator_progress',
        payload: {
          percent: 40,
          current_step: 'architect',
          completed_nodes: 4,
          total_nodes: 10,
        },
      }),
    ];
    const html = render(h(ProgressBar, { events, frozen: false }));
    expect(html).toContain('4/10 nodes');
  });

  test('renders per-agent rows from agent_progress', () => {
    const events = [
      ev({
        type: 'agent_progress',
        actor: { kind: 'agent', name: 'scout' },
        payload: { agent: 'scout', percent: 20, current_step: 'scan' },
      }),
      ev({
        type: 'agent_progress',
        actor: { kind: 'agent', name: 'scout' },
        payload: { agent: 'scout', percent: 60, current_step: 'summarize' },
      }),
    ];
    const html = render(h(ProgressBar, { events, frozen: false }));
    expect(html).toContain('scout');
    expect(html).toContain('60%');
  });

  test('frozen + not-completed forces 100%', () => {
    const events: BenderEvent[] = [];
    const html = render(h(ProgressBar, { events, frozen: true }));
    expect(html).toContain('>100<');
  });
});

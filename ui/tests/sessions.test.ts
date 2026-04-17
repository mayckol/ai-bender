import { describe, expect, test } from 'bun:test';
import { mkdtemp, mkdir, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import {
  computeDuration,
  exportSession,
  listSessions,
  parseEventsJSONL,
  reportPath,
  sessionDir,
  sessionsRoot,
  summarizeEvents,
} from '../src/server/sessions.ts';

async function fixture() {
  const root = await mkdtemp(join(tmpdir(), 'bender-ui-'));
  const id = '2026-04-16T22-04-07-f6g';
  const dir = join(root, '.bender', 'sessions', id);
  await mkdir(dir, { recursive: true });

  const state = {
    schema_version: 1,
    session_id: id,
    command: '/ghu',
    started_at: '2026-04-16T22:04:07Z',
    completed_at: '2026-04-16T22:04:28Z',
    status: 'completed',
    source_artifacts: ['.bender/artifacts/specs/x.md'],
    skills_invoked: [],
    files_changed: 3,
    findings_count: 3,
  };
  const events = [
    {
      schema_version: 1,
      session_id: id,
      timestamp: '2026-04-16T22:04:07Z',
      actor: { kind: 'orchestrator', name: 'ghu' },
      type: 'session_started',
      payload: { command: '/ghu', invoker: 'u', working_dir: '/x' },
    },
    {
      schema_version: 1,
      session_id: id,
      timestamp: '2026-04-16T22:04:28Z',
      actor: { kind: 'orchestrator', name: 'ghu' },
      type: 'session_completed',
      payload: { status: 'completed', duration_ms: 21000 },
    },
  ];

  await writeFile(join(dir, 'state.json'), JSON.stringify(state));
  await writeFile(join(dir, 'events.jsonl'), events.map((e) => JSON.stringify(e)).join('\n') + '\n');
  return { root, id, dir, state, events };
}

describe('sessions module', () => {
  test('sessionsRoot joins .bender/sessions', () => {
    expect(sessionsRoot('/tmp/p')).toBe('/tmp/p/.bender/sessions');
  });

  test('sessionDir builds the full path', () => {
    expect(sessionDir('/tmp/p', 'abc')).toBe('/tmp/p/.bender/sessions/abc');
  });

  test('listSessions returns an empty slice when the root is missing', async () => {
    const empty = await mkdtemp(join(tmpdir(), 'bender-ui-empty-'));
    const list = await listSessions(empty);
    expect(list).toEqual([]);
  });

  test('listSessions parses state + duration + agents + skills from on-disk fixtures', async () => {
    const f = await fixture();
    const list = await listSessions(f.root);
    expect(list).toHaveLength(1);
    expect(list[0].id).toBe(f.id);
    expect(list[0].state.command).toBe('/ghu');
    expect(list[0].duration_ms).toBe(21000);
    // Fixture only has session_started + session_completed with orchestrator
    // actors, so both collapse to the "main" attribution.
    expect(list[0].agents).toEqual(['main']);
    expect(list[0].skills).toEqual([]);
  });

  test('summarizeEvents threads agents and skills by responsibility', () => {
    const events = [
      { schema_version: 1, session_id: 's1', timestamp: 't', actor: { kind: 'orchestrator', name: 'ghu' }, type: 'session_started', payload: { command: '/ghu' } },
      { schema_version: 1, session_id: 's1', timestamp: 't', actor: { kind: 'agent', name: 'crafter' }, type: 'skill_invoked', payload: { skill: 'bg-crafter-implement', agent: 'crafter' } },
      { schema_version: 1, session_id: 's1', timestamp: 't', actor: { kind: 'agent', name: 'tester' }, type: 'skill_invoked', payload: { skill: 'bg-tester-write-and-run', agent: 'tester' } },
      { schema_version: 1, session_id: 's1', timestamp: 't', actor: { kind: 'orchestrator', name: 'ghu' }, type: 'orchestrator_decision', payload: { decision_type: 'agent_assignment', dispatched_agent: 'crafter' } },
    ] as any;
    const sum = summarizeEvents(events);
    expect(sum.agents).toEqual(['crafter', 'main', 'tester']);
    expect(sum.skills).toEqual(['bg-crafter-implement', 'bg-tester-write-and-run']);
  });

  test('exportSession returns state + events parity with on-disk files', async () => {
    const f = await fixture();
    const snap = await exportSession(f.dir);
    expect(snap.state.session_id).toBe(f.id);
    expect(snap.events).toHaveLength(2);
    expect(snap.events[0].type).toBe('session_started');
  });

  test('parseEventsJSONL skips empty + malformed lines', () => {
    const raw = [
      '{"type":"session_started","actor":{"kind":"user","name":"u"},"schema_version":1,"session_id":"s","timestamp":"t"}',
      '',
      'not json',
      '{"type":"session_completed","actor":{"kind":"user","name":"u"},"schema_version":1,"session_id":"s","timestamp":"t","payload":{"status":"completed","duration_ms":0}}',
    ].join('\n');
    const out = parseEventsJSONL(raw);
    expect(out).toHaveLength(2);
    expect(out[0].type).toBe('session_started');
    expect(out[1].type).toBe('session_completed');
  });

  test('computeDuration returns 0 when there are no events', () => {
    expect(computeDuration('2026-04-16T22:04:07Z', [])).toBe(0);
  });

  test('reportPath strips the id suffix to the timestamp head', () => {
    const p = reportPath('/proj', '2026-04-16T22-04-07-f6g');
    expect(p).toBe('/proj/.bender/artifacts/ghu/run-2026-04-16T22-04-07-report.md');
  });
});

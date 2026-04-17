import { useEffect, useMemo, useState } from 'preact/hooks';

import { agentColor } from '../lib/agents.ts';
import type { BenderEvent, SessionExport, SessionState, SessionSummary } from '../lib/api.ts';
import {
  buildFlowWaves,
  pickLiveGhu,
  type FlowNode,
  type FlowWave,
  type NodeState,
} from '../lib/project-flow.ts';
import { subscribeSSE } from '../lib/sse.ts';

interface Props {
  sessions: SessionSummary[];
}

export function LiveProjectFlow({ sessions }: Props) {
  const liveGhu = useMemo(() => pickLiveGhu(sessions), [sessions]);

  const [events, setEvents] = useState<BenderEvent[]>([]);
  const [state, setState] = useState<SessionState | null>(null);
  const liveId = liveGhu?.id ?? null;
  const isLive = liveGhu?.state.status === 'running';

  useEffect(() => {
    if (!liveId) {
      setEvents([]);
      setState(null);
      return;
    }
    setEvents([]);
    setState(null);
    const stop = subscribeSSE(`/api/sessions/${encodeURIComponent(liveId)}/stream`, {
      handlers: {
        snapshot: (ev) => {
          try {
            const snap = JSON.parse(ev.data) as SessionExport;
            setState(snap.state);
            setEvents(snap.events);
          } catch { /* keep current */ }
        },
        event: (ev) => {
          try {
            const parsed = JSON.parse(ev.data) as BenderEvent;
            setEvents((prev) => [...prev, parsed]);
          } catch { /* skip malformed */ }
        },
        'state-patch': (ev) => {
          try { setState(JSON.parse(ev.data) as SessionState); } catch { /* skip */ }
        },
      },
    });
    return stop;
  }, [liveId]);

  const waves = useMemo(() => buildFlowWaves(events, state, isLive), [events, state, isLive]);

  return (
    <div class="pfchain-frame">
      {liveGhu ? (
        <header class="pfchain-head">
          <div class="pfchain-kicker">{isLive ? 'LIVE BACKGROUND RUN' : 'LAST RUN'}</div>
          <div class="pfchain-title">
            <a href={`/sessions/${liveGhu.id}`}>
              <code>{liveGhu.state.command}</code> · {liveGhu.id}
            </a>
          </div>
          <div class="pfchain-meta">
            <WaveStats waves={waves} />
          </div>
        </header>
      ) : (
        <header class="pfchain-head pfchain-head-idle">
          <div class="pfchain-kicker">IDLE</div>
          <div class="pfchain-title">No <code>/ghu</code> run yet — anchors show the conceptual flow.</div>
        </header>
      )}

      <ol class="pfchain" role="list">
        {waves.map((wave, idx) => (
          <li key={wave.id} class={`pfchain-slot${wave.parallel ? ' is-parallel' : ''}`}>
            <WaveCell wave={wave} />
            {idx < waves.length - 1 && (
              <Connector fromState={slotState(wave)} />
            )}
          </li>
        ))}
      </ol>
    </div>
  );
}

function WaveStats({ waves }: { waves: FlowWave[] }) {
  const running = waves.flatMap((w) => w.nodes).filter((n) => n.state === 'running').length;
  const parallelCount = waves.filter((w) => w.parallel).length;
  return (
    <div class="pfchain-stats">
      <StatChip label="running" value={String(running).padStart(2, '0')} tone={running > 0 ? 'signal' : 'muted'} />
      <StatChip label="parallel" value={String(parallelCount).padStart(2, '0')} tone={parallelCount > 0 ? 'phosphor' : 'muted'} />
      <StatChip label="waves" value={String(waves.length).padStart(2, '0')} tone="muted" />
    </div>
  );
}

function StatChip({ label, value, tone }: { label: string; value: string; tone: 'signal' | 'phosphor' | 'muted' }) {
  return (
    <div class={`pf-stat tone-${tone}`}>
      <div class="pf-stat-key">{label}</div>
      <div class="pf-stat-val">{value}</div>
    </div>
  );
}

function WaveCell({ wave }: { wave: FlowWave }) {
  if (wave.parallel) {
    return (
      <div class="pfwave pfwave-parallel">
        <div class="pfwave-brace" aria-hidden="true">
          <span class="pfwave-brace-label">PARALLEL · {wave.nodes.length}</span>
          <span class="pfwave-brace-pulse" />
        </div>
        <div class="pfwave-stack">
          {wave.nodes.map((n) => <FlowNodeCard key={n.id} node={n} dense />)}
        </div>
      </div>
    );
  }
  const [node] = wave.nodes;
  return (
    <div class="pfwave pfwave-solo">
      <FlowNodeCard node={node} />
    </div>
  );
}

function FlowNodeCard({ node, dense }: { node: FlowNode; dense?: boolean }) {
  const color = node.isAnchor || node.agent === 'ship' ? 'var(--signal)' : agentColor(node.agent);
  const glyph = glyphFor(node);
  return (
    <div
      class={`pfnode pfnode-${node.state}${node.isAnchor ? ' is-anchor' : ''}${dense ? ' is-dense' : ''}`}
      style={node.state !== 'disabled' && node.state !== 'failed' ? { ['--pfnode-accent' as string]: color } : undefined}
    >
      <span class="pfnode-tick pfnode-tick-tl" aria-hidden="true" />
      <span class="pfnode-tick pfnode-tick-tr" aria-hidden="true" />
      <span class="pfnode-tick pfnode-tick-bl" aria-hidden="true" />
      <span class="pfnode-tick pfnode-tick-br" aria-hidden="true" />
      {node.state === 'running' && (
        <>
          <span class="pfnode-ring pfnode-ring-outer" aria-hidden="true" />
          <span class="pfnode-ring pfnode-ring-inner" aria-hidden="true" />
          <span class="pfnode-scan" aria-hidden="true" />
          <span class="pfnode-heart" aria-hidden="true" />
        </>
      )}
      <div class="pfnode-glyph" aria-hidden="true">{glyph}</div>
      <div class="pfnode-name">{node.agent}</div>
      {node.skill && <div class="pfnode-skill">{node.skill}</div>}
      <NodeStatusChip node={node} />
    </div>
  );
}

function NodeStatusChip({ node }: { node: FlowNode }) {
  const label = chipText(node);
  return (
    <div class={`pfnode-chip chip-${node.state}`}>
      <span class="pfnode-chip-dot" aria-hidden="true" />
      <span class="pfnode-chip-text">{label}</span>
      {node.state === 'running' && node.startedAt && (
        <ElapsedCounter from={node.startedAt} />
      )}
    </div>
  );
}

function Connector({ fromState }: { fromState: NodeState }) {
  const cls =
    fromState === 'running' ? 'pfconn-active' :
    fromState === 'completed' ? 'pfconn-done' :
    fromState === 'failed' ? 'pfconn-failed' :
    'pfconn-idle';
  return (
    <div class={`pfconn ${cls}`} aria-hidden="true">
      <span class="pfconn-beam" />
      <span class="pfconn-head">›</span>
    </div>
  );
}

function ElapsedCounter({ from }: { from: string }) {
  const start = useMemo(() => Date.parse(from) || Date.now(), [from]);
  const [, setTick] = useState(0);
  useEffect(() => {
    const id = setInterval(() => setTick((x) => x + 1), 1000);
    return () => clearInterval(id);
  }, []);
  const elapsed = Math.max(0, Date.now() - start);
  return <span class="pfnode-chip-elapsed">{fmtElapsed(elapsed)}</span>;
}

function slotState(wave: FlowWave): NodeState {
  if (wave.nodes.some((n) => n.state === 'running')) return 'running';
  if (wave.nodes.some((n) => n.state === 'failed')) return 'failed';
  if (wave.nodes.every((n) => n.state === 'completed')) return 'completed';
  if (wave.nodes.some((n) => n.state === 'blocked')) return 'blocked';
  return 'disabled';
}

function chipText(node: FlowNode): string {
  switch (node.state) {
    case 'running':   return node.isAnchor && node.agent === 'ship' ? 'awaiting' : 'running';
    case 'completed': return node.agent === 'ship' ? 'shipped' : 'done';
    case 'failed':    return 'failed';
    case 'blocked':   return 'blocked';
    case 'disabled':  return node.isAnchor && node.agent === 'ship' ? 'idle' : 'queued';
  }
}

function glyphFor(node: FlowNode): string {
  if (node.agent === 'ship') return '▲';
  if (node.agent === 'crafter') return '◆';
  if (node.agent === 'tester') return '◉';
  if (node.agent === 'scout') return '◇';
  if (node.agent === 'architect') return '◈';
  if (node.agent === 'reviewer') return '▣';
  if (node.agent === 'sentinel') return '◉';
  if (node.agent === 'benchmarker') return '◎';
  if (node.agent === 'scribe') return '□';
  if (node.agent === 'linter') return '◇';
  if (node.agent === 'surgeon') return '◆';
  return '●';
}

function fmtElapsed(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  const rem = s - m * 60;
  return `${m}m${rem.toString().padStart(2, '0')}s`;
}

import type { ComponentChildren } from 'preact';
import { useEffect, useMemo, useRef, useState } from 'preact/hooks';

import { agentColor, responsibleAgent } from '../lib/agents.ts';
import type { BenderEvent, SessionExport, SessionState, SessionSummary } from '../lib/api.ts';
import {
  buildFlowWaves,
  buildSessionWaves,
  pickLiveGhu,
  type FlowNode,
  type FlowWave,
  type NodeState,
} from '../lib/project-flow.ts';
import { subscribeSSE } from '../lib/sse.ts';
import { EventRow } from './EventRow.tsx';

interface Props {
  sessions: SessionSummary[];
}

/**
 * Idempotency signature for a single event — two events producing the same
 * key are treated as duplicates and dropped at ingest. Guards against
 * SSE snapshot + tail overlap and any upstream re-emission.
 */
function flowEventKey(ev: BenderEvent): string {
  const p = (ev.payload ?? {}) as Record<string, unknown>;
  const agent = typeof p.agent === 'string' ? p.agent : '';
  const skill = typeof p.skill === 'string' ? p.skill : '';
  const decision = typeof p.decision_type === 'string' ? p.decision_type : '';
  const dispatched = Array.isArray(p.dispatched_agents) ? (p.dispatched_agents as unknown[]).join(',') : '';
  return `${ev.timestamp}|${ev.type}|${ev.actor?.kind ?? ''}|${ev.actor?.name ?? ''}|${agent}|${skill}|${decision}|${dispatched}`;
}

/**
 * Stages-list variant: picks the latest /ghu or /implement session from the
 * sessions list, subscribes to its SSE stream, and renders the flow.
 */
export function LiveProjectFlow({ sessions }: Props) {
  const liveGhu = useMemo(() => pickLiveGhu(sessions), [sessions]);

  const [events, setEvents] = useState<BenderEvent[]>([]);
  const [state, setState] = useState<SessionState | null>(null);
  const liveId = liveGhu?.id ?? null;
  const isLive = liveGhu?.state.status === 'running';

  const seenRef = useRef<Set<string>>(new Set());

  useEffect(() => {
    if (!liveId) {
      setEvents([]);
      setState(null);
      seenRef.current = new Set();
      return;
    }
    setEvents([]);
    setState(null);
    seenRef.current = new Set();
    const stop = subscribeSSE(`/api/sessions/${encodeURIComponent(liveId)}/stream`, {
      handlers: {
        snapshot: (ev) => {
          try {
            const snap = JSON.parse(ev.data) as SessionExport;
            setState(snap.state);
            const seen = new Set<string>();
            const deduped: BenderEvent[] = [];
            for (const e of snap.events) {
              const k = flowEventKey(e);
              if (seen.has(k)) continue;
              seen.add(k);
              deduped.push(e);
            }
            seenRef.current = seen;
            setEvents(deduped);
          } catch { /* keep current */ }
        },
        event: (ev) => {
          try {
            const parsed = JSON.parse(ev.data) as BenderEvent;
            const k = flowEventKey(parsed);
            if (seenRef.current.has(k)) return;
            seenRef.current.add(k);
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

  const header = liveGhu
    ? {
        kicker: isLive ? 'LIVE BACKGROUND RUN' : 'LAST RUN',
        title: (
          <a href={`/sessions/${liveGhu.id}`}>
            <code>{liveGhu.state.command}</code> · {liveGhu.id}
          </a>
        ),
        idle: false,
      }
    : {
        kicker: 'IDLE',
        title: <>No <code>/ghu</code> run yet — anchors show the conceptual flow.</>,
        idle: true,
      };

  const waves = useMemo(() => buildFlowWaves(events, state, isLive), [events, state, isLive]);

  return (
    <FlowCanvas
      events={events}
      waves={waves}
      header={header}
      resetKey={liveId}
    />
  );
}

/**
 * Single-session variant used on the Timeline page. Takes events + state
 * straight from the Timeline's own SSE subscription; no header (Timeline
 * provides the "002 PIPELINE" card frame and kicker itself).
 */
interface SessionAgentFlowProps {
  events: BenderEvent[];
  state: SessionState | null;
}

export function SessionAgentFlow({ events, state }: SessionAgentFlowProps) {
  const waves = useMemo(() => buildSessionWaves(events, state), [events, state]);
  return (
    <FlowCanvas
      events={events}
      waves={waves}
      header={null}
      resetKey={state?.session_id ?? null}
      agentTinted
    />
  );
}

/* ------------------------------------------------------------------ */
/* Shared canvas — header + grid + chain + dive                        */
/* ------------------------------------------------------------------ */
interface FlowCanvasHeader {
  kicker: string;
  title: ComponentChildren;
  idle: boolean;
}

interface FlowCanvasProps {
  events: BenderEvent[];
  waves: FlowWave[];
  header: FlowCanvasHeader | null;
  resetKey: string | null;
  agentTinted?: boolean;
}

function FlowCanvas({ events, waves, header, resetKey, agentTinted }: FlowCanvasProps) {
  const [diveAgent, setDiveAgent] = useState<string | null>(null);
  const zoom = useFlowZoom();

  // Collapse dive state when the tracked session changes.
  useEffect(() => { setDiveAgent(null); }, [resetKey]);

  if (diveAgent) {
    return (
      <NodeDive
        agent={diveAgent}
        events={events}
        waves={waves}
        onBack={() => setDiveAgent(null)}
      />
    );
  }

  return (
    <div class="pfchain-frame">
      {header && (
        <header class={`pfchain-head${header.idle ? ' pfchain-head-idle' : ''}`}>
          <div class="pfchain-kicker">{header.kicker}</div>
          <div class="pfchain-title">{header.title}</div>
          <div class="pfchain-meta">
            <WaveStats waves={waves} />
            <ZoomControls zoom={zoom} />
          </div>
        </header>
      )}

      {!header && (
        <div class="pfchain-toolbar">
          <WaveStats waves={waves} />
          <ZoomControls zoom={zoom} />
        </div>
      )}

      <div class="pfchain-canvas" ref={zoom.attachContainer}>
        <span class="pfcanvas-bracket pfcanvas-bracket-tl" aria-hidden="true" />
        <span class="pfcanvas-bracket pfcanvas-bracket-tr" aria-hidden="true" />
        <span class="pfcanvas-bracket pfcanvas-bracket-bl" aria-hidden="true" />
        <span class="pfcanvas-bracket pfcanvas-bracket-br" aria-hidden="true" />
        <div class="pfcanvas-axis" aria-hidden="true">
          <span class="pfcanvas-axis-label">XY · 01</span>
        </div>
        <div class="pfcanvas-zoom-hint" aria-hidden="true">scroll to zoom · drag to pan · ⌘ for bigger steps</div>
        <ol
          class="pfchain"
          ref={zoom.attachChain}
          role="list"
          style={{
            ['--pf-scale' as string]: String(zoom.scale),
            ['--pf-tx' as string]: `${zoom.tx}px`,
            ['--pf-ty' as string]: `${zoom.ty}px`,
          }}
        >
          {waves.map((wave, idx) => (
            <li key={wave.id} class={`pfchain-slot${wave.parallel ? ' is-parallel' : ''}`}>
              <WaveCell wave={wave} onDive={setDiveAgent} agentTinted={agentTinted} />
              {idx < waves.length - 1 && (
                <Connector fromState={slotState(wave)} />
              )}
            </li>
          ))}
        </ol>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Zoom / fit controller                                               */
/* ------------------------------------------------------------------ */

const MIN_SCALE = 0.4;
const MAX_SCALE = 1.5;

interface FlowZoom {
  scale: number;
  mode: 'fit' | 'manual';
  tx: number;
  ty: number;
  attachContainer: (el: HTMLDivElement | null) => void;
  attachChain: (el: HTMLOListElement | null) => void;
  fit: () => void;
  zoomIn: () => void;
  zoomOut: () => void;
  reset: () => void;
}

function useFlowZoom(): FlowZoom {
  const containerElRef = useRef<HTMLDivElement | null>(null);
  const chainElRef = useRef<HTMLOListElement | null>(null);
  const scaleRef = useRef(1);
  const txRef = useRef(0);
  const tyRef = useRef(0);
  const roRef = useRef<ResizeObserver | null>(null);
  const wheelRef = useRef<((e: WheelEvent) => void) | null>(null);
  const pointerDownRef = useRef<((e: PointerEvent) => void) | null>(null);
  const pointerMoveRef = useRef<((e: PointerEvent) => void) | null>(null);
  const pointerUpRef = useRef<((e: PointerEvent) => void) | null>(null);
  const dragStateRef = useRef<{ x: number; y: number; startTx: number; startTy: number; pointerId: number } | null>(null);
  const [mode, setMode] = useState<'fit' | 'manual'>('fit');
  const [scale, setScaleState] = useState(1);
  const [tx, setTxState] = useState(0);
  const [ty, setTyState] = useState(0);
  const modeRef = useRef<'fit' | 'manual'>('fit');

  const setScale = (v: number) => {
    scaleRef.current = v;
    setScaleState(v);
  };
  const setTranslation = (x: number, y: number) => {
    txRef.current = x;
    tyRef.current = y;
    setTxState(x);
    setTyState(y);
  };
  const step = (delta: number) => {
    modeRef.current = 'manual';
    setMode('manual');
    const next = Math.max(MIN_SCALE, Math.min(MAX_SCALE, Number((scaleRef.current + delta).toFixed(3))));
    setScale(next);
  };

  // Fit-to-container calculation. Runs whenever the container or chain
  // resizes, and respects the user's manual scale when they've touched +/-.
  const calcFit = () => {
    const container = containerElRef.current;
    const chain = chainElRef.current;
    if (!container || !chain) return;
    if (modeRef.current !== 'fit') return;
    const containerW = container.clientWidth - 36; // padding allowance
    const natural = chain.scrollWidth / (scaleRef.current || 1);
    if (natural <= 0 || containerW <= 0) return;
    const next = Math.max(MIN_SCALE, Math.min(1, containerW / natural));
    setScale(Number(next.toFixed(3)));
  };

  // Keep modeRef in sync for the observer + wheel handlers that read it
  // without going through React state.
  useEffect(() => {
    modeRef.current = mode;
    if (mode === 'fit') calcFit();
  }, [mode]);

  // Callback refs — attach listeners and observers exactly when the element
  // mounts (and tear down when it unmounts). Avoids the timing fragility of
  // relying on a useEffect to pick up a ref that "should be" populated.
  const attachContainer = (el: HTMLDivElement | null) => {
    // Detach from prior element.
    const prev = containerElRef.current;
    if (prev) {
      if (wheelRef.current) prev.removeEventListener('wheel', wheelRef.current);
      if (pointerDownRef.current) prev.removeEventListener('pointerdown', pointerDownRef.current);
      if (pointerMoveRef.current) prev.removeEventListener('pointermove', pointerMoveRef.current);
      if (pointerUpRef.current) {
        prev.removeEventListener('pointerup', pointerUpRef.current);
        prev.removeEventListener('pointercancel', pointerUpRef.current);
      }
    }
    containerElRef.current = el;
    if (!el) return;

    // --- Wheel (zoom) ---
    const onWheel = (e: WheelEvent) => {
      // Plain wheel zooms (Figma-style); Cmd/Ctrl multiplies the step so
      // trackpad pinch (ctrlKey=true) feels equivalent to mouse-wheel.
      e.preventDefault();
      const coarse = e.ctrlKey || e.metaKey;
      const base = coarse ? 0.12 : 0.06;
      const delta = e.deltaY < 0 ? base : -base;
      step(delta);
    };
    wheelRef.current = onWheel;
    el.addEventListener('wheel', onWheel, { passive: false });

    // --- Pointer drag (pan) ---
    const isInteractive = (target: EventTarget | null): boolean => {
      if (!(target instanceof Element)) return false;
      return !!target.closest('a, button, input, select, textarea, details > summary, .pfzoom, .pfnode-dive');
    };
    const onPointerDown = (e: PointerEvent) => {
      if (e.button !== 0 && e.pointerType === 'mouse') return;
      if (isInteractive(e.target)) return;
      dragStateRef.current = {
        x: e.clientX,
        y: e.clientY,
        startTx: txRef.current,
        startTy: tyRef.current,
        pointerId: e.pointerId,
      };
      try { el.setPointerCapture(e.pointerId); } catch { /* older browsers */ }
      el.classList.add('is-panning');
    };
    const onPointerMove = (e: PointerEvent) => {
      const drag = dragStateRef.current;
      if (!drag || e.pointerId !== drag.pointerId) return;
      const dx = e.clientX - drag.x;
      const dy = e.clientY - drag.y;
      setTranslation(drag.startTx + dx, drag.startTy + dy);
      if (modeRef.current !== 'manual') {
        modeRef.current = 'manual';
        setMode('manual');
      }
    };
    const onPointerUp = (e: PointerEvent) => {
      const drag = dragStateRef.current;
      if (!drag) return;
      if (e.pointerId !== drag.pointerId) return;
      try { el.releasePointerCapture(e.pointerId); } catch { /* ignore */ }
      dragStateRef.current = null;
      el.classList.remove('is-panning');
    };
    pointerDownRef.current = onPointerDown;
    pointerMoveRef.current = onPointerMove;
    pointerUpRef.current = onPointerUp;
    el.addEventListener('pointerdown', onPointerDown);
    el.addEventListener('pointermove', onPointerMove);
    el.addEventListener('pointerup', onPointerUp);
    el.addEventListener('pointercancel', onPointerUp);

    // (Re)wire the resize observer.
    if (roRef.current) roRef.current.disconnect();
    const ro = new ResizeObserver(() => calcFit());
    ro.observe(el);
    if (chainElRef.current) ro.observe(chainElRef.current);
    roRef.current = ro;
  };

  const attachChain = (el: HTMLOListElement | null) => {
    chainElRef.current = el;
    if (!el || !roRef.current) return;
    roRef.current.observe(el);
    calcFit();
  };

  return {
    scale,
    mode,
    tx,
    ty,
    attachContainer,
    attachChain,
    fit: () => {
      modeRef.current = 'fit';
      setMode('fit');
      setTranslation(0, 0);
    },
    zoomIn: () => step(0.1),
    zoomOut: () => step(-0.1),
    reset: () => {
      modeRef.current = 'manual';
      setMode('manual');
      setScale(1);
      setTranslation(0, 0);
    },
  };
}

function ZoomControls({ zoom }: { zoom: FlowZoom }) {
  const pct = Math.round(zoom.scale * 100);
  return (
    <div class="pfzoom" role="group" aria-label="Zoom flow">
      <button
        type="button"
        class="pfzoom-btn"
        onClick={zoom.zoomOut}
        aria-label="Zoom out"
        title="Zoom out"
        disabled={zoom.scale <= MIN_SCALE + 0.001}
      >−</button>
      <button
        type="button"
        class={`pfzoom-mode${zoom.mode === 'fit' ? ' is-active' : ''}`}
        onClick={zoom.fit}
        title="Fit to width"
      >FIT</button>
      <div class="pfzoom-val" aria-live="polite">{pct}%</div>
      <button
        type="button"
        class="pfzoom-btn"
        onClick={zoom.reset}
        aria-label="Reset to 100%"
        title="Reset (100%)"
      >1:1</button>
      <button
        type="button"
        class="pfzoom-btn"
        onClick={zoom.zoomIn}
        aria-label="Zoom in"
        title="Zoom in"
        disabled={zoom.scale >= MAX_SCALE - 0.001}
      >+</button>
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

function WaveCell({ wave, onDive, agentTinted }: { wave: FlowWave; onDive?: (agent: string) => void; agentTinted?: boolean }) {
  if (wave.parallel) {
    return (
      <div class="pfwave pfwave-parallel">
        <div class="pfwave-brace" aria-hidden="true">
          <span class="pfwave-brace-label">PARALLEL · {wave.nodes.length}</span>
          <span class="pfwave-brace-pulse" />
        </div>
        <div class="pfwave-stack">
          {wave.nodes.map((n) => <FlowNodeCard key={n.id} node={n} dense onDive={onDive} agentTinted={agentTinted} />)}
        </div>
      </div>
    );
  }
  const [node] = wave.nodes;
  return (
    <div class="pfwave pfwave-solo">
      <FlowNodeCard node={node} onDive={onDive} agentTinted={agentTinted} />
    </div>
  );
}

function FlowNodeCard({
  node,
  dense,
  onDive,
  agentTinted,
}: {
  node: FlowNode;
  dense?: boolean;
  onDive?: (agent: string) => void;
  agentTinted?: boolean;
}) {
  const color = node.isAnchor || node.agent === 'ship' ? 'var(--signal)' : agentColor(node.agent);
  const glyph = glyphFor(node);
  // Dive only applies to actual agent nodes that have landed real events
  // (skip the ship anchor; allow crafter even as anchor if it's been seen).
  const canDive = !!onDive && node.agent !== 'ship' && node.state !== 'disabled';
  return (
    <div
      class={`pfnode pfnode-${node.state}${node.isAnchor ? ' is-anchor' : ''}${dense ? ' is-dense' : ''}${canDive ? ' can-dive' : ''}${agentTinted ? ' is-agent-tinted' : ''}`}
      style={agentTinted || (node.state !== 'disabled' && node.state !== 'failed') ? { ['--pfnode-accent' as string]: color } : undefined}
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
      {canDive && (
        <button
          type="button"
          class="pfnode-dive"
          title={`Dive into ${node.agent}`}
          aria-label={`Dive into ${node.agent}`}
          onPointerDown={(e) => e.stopPropagation()}
          onClick={(e) => { e.stopPropagation(); onDive!(node.agent); }}
        >
          <span class="pfnode-dive-icon" aria-hidden="true">⤢</span>
          <span class="pfnode-dive-label">dive</span>
        </button>
      )}
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

/* ------------------------------------------------------------------ */
/* Dive view — one node's events as a timeline                         */
/* ------------------------------------------------------------------ */
function NodeDive({
  agent,
  events,
  waves,
  onBack,
}: {
  agent: string;
  events: BenderEvent[];
  waves: FlowWave[];
  onBack: () => void;
}) {
  const agentEvents = useMemo(() => {
    const out: BenderEvent[] = [];
    for (const ev of events) {
      if (responsibleAgent(ev) === agent) out.push(ev);
    }
    return out;
  }, [events, agent]);

  const node = useMemo(() => {
    for (const w of waves) {
      for (const n of w.nodes) {
        if (n.agent === agent) return n;
      }
    }
    return null;
  }, [waves, agent]);

  const counts = useMemo(() => {
    const c = { skills: 0, files: 0, findings: 0, progress: 0, started: 0, completed: 0, failed: 0 };
    for (const ev of agentEvents) {
      if (ev.type === 'skill_invoked') c.skills++;
      else if (ev.type === 'file_changed') c.files++;
      else if (ev.type === 'finding_reported') c.findings++;
      else if (ev.type === 'agent_progress') c.progress++;
      else if (ev.type === 'agent_started') c.started++;
      else if (ev.type === 'agent_completed') c.completed++;
      else if (ev.type === 'agent_failed' || ev.type === 'skill_failed') c.failed++;
    }
    return c;
  }, [agentEvents]);

  const accent = agentColor(agent);
  const glyph = node ? glyphFor(node) : '●';
  const status = node?.state ?? 'disabled';

  return (
    <div class="pfdive">
      <header class="pfdive-head">
        <button type="button" class="pfdive-back" onClick={onBack}>
          <span class="pfdive-back-icon" aria-hidden="true">◂</span>
          back to flow
        </button>
        <div class="pfdive-head-rule" aria-hidden="true" />
        <div class={`pfdive-badge pfdive-badge-${status}`} style={{ ['--pfnode-accent' as string]: accent }}>
          <span class="pfdive-badge-glyph" aria-hidden="true">{glyph}</span>
          <div class="pfdive-badge-body">
            <div class="pfdive-badge-name">{agent}</div>
            {node?.skill && <div class="pfdive-badge-skill">{node.skill}</div>}
          </div>
          <span class={`pfdive-badge-chip chip-${status}`}>
            <span class="pfnode-chip-dot" aria-hidden="true" />
            {status}
          </span>
        </div>
      </header>

      <div class="pfdive-stats">
        <DiveStat label="skills" value={counts.skills} />
        <DiveStat label="files" value={counts.files} />
        <DiveStat label="findings" value={counts.findings} tone={counts.findings > 0 ? 'warn' : undefined} />
        <DiveStat label="progress" value={counts.progress} />
        <DiveStat label="events" value={agentEvents.length} tone="signal" />
      </div>

      {agentEvents.length === 0 ? (
        <div class="pfdive-empty">
          <div class="pfdive-empty-rule" aria-hidden="true" />
          <p>No events have landed for <code>{agent}</code> yet.</p>
        </div>
      ) : (
        <div class="pfdive-timeline">
          {agentEvents.map((ev, i) => <EventRow key={i} event={ev} />)}
        </div>
      )}
    </div>
  );
}

function DiveStat({ label, value, tone }: { label: string; value: number; tone?: string }) {
  return (
    <div class={`pfdive-stat${tone ? ` tone-${tone}` : ''}`}>
      <div class="pfdive-stat-key">{label}</div>
      <div class="pfdive-stat-val">{String(value).padStart(2, '0')}</div>
    </div>
  );
}

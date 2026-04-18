import { useEffect, useMemo, useState } from 'preact/hooks';

import { agentColor } from '../lib/agents.ts';
import type { BenderEvent, SessionState } from '../lib/api.ts';

interface Props {
  events: BenderEvent[];
  state: SessionState | null;
}

interface Interval {
  start: number;
  end: number;
  ongoing: boolean;
}

interface LaneData {
  agent: string;
  color: string;
  intervals: Interval[];
  total: number;
  firstStart: number;
}

/**
 * Top-down swimlane / Gantt — time flows down the Y axis; each agent gets
 * its own vertical lane (column). Bars show when that agent was active;
 * overlapping Y ranges across different lanes = parallel execution.
 */
export function SessionAgentGraph({ events, state }: Props) {
  const now = useNow(state?.status === 'running');

  const lanes = useMemo(() => buildLanes(events), [events]);

  const { tMin, tMax, duration } = useMemo(() => {
    if (lanes.length === 0) return { tMin: 0, tMax: 0, duration: 0 };
    let lo = Infinity;
    let hi = 0;
    for (const lane of lanes) {
      for (const iv of lane.intervals) {
        if (iv.start < lo) lo = iv.start;
        const end = iv.ongoing ? now : iv.end;
        if (end > hi) hi = end;
      }
    }
    if (!Number.isFinite(lo)) lo = 0;
    const d = Math.max(1, hi - lo);
    return { tMin: lo, tMax: hi, duration: d };
  }, [lanes, now]);

  const ticks = useMemo(() => buildTicks(duration), [duration]);
  const graphHeight = Math.max(220, Math.min(900, Math.floor(duration / 1000) * 14));

  if (lanes.length === 0) {
    return (
      <div class="pfgraph-empty">
        <div class="pfgraph-empty-rule" aria-hidden="true" />
        <p>No agent activity yet. Bars will appear as agents emit <code>agent_started</code> events.</p>
      </div>
    );
  }

  const parallelBands = useMemo(() => buildParallelBands(lanes, tMin, duration), [lanes, tMin, duration]);

  return (
    <div class="pfgraph">
      <header class="pfgraph-head">
        <div class="pfgraph-head-kicker">swimlane · {lanes.length} agents · {fmtDuration(duration)} elapsed</div>
        <div class="pfgraph-head-legend">
          <span class="pfgraph-legend-chip">
            <span class="pfgraph-legend-dot legend-done" /> completed
          </span>
          <span class="pfgraph-legend-chip">
            <span class="pfgraph-legend-dot legend-running" /> running
          </span>
          <span class="pfgraph-legend-chip">
            <span class="pfgraph-legend-dot legend-failed" /> failed
          </span>
          <span class="pfgraph-legend-chip">
            <span class="pfgraph-legend-dash" /> parallel band
          </span>
        </div>
      </header>

      <div class="pfgraph-body">
        <div class="pfgraph-axis" style={{ height: `${graphHeight}px` }}>
          {ticks.map((t) => (
            <div
              key={t.ms}
              class="pfgraph-tick"
              style={{ top: `${(t.ms / duration) * graphHeight}px` }}
            >
              <span class="pfgraph-tick-label">{t.label}</span>
              <span class="pfgraph-tick-line" aria-hidden="true" />
            </div>
          ))}
        </div>

        <div class="pfgraph-lanes-wrap">
          <div class="pfgraph-lane-heads">
            {lanes.map((l) => (
              <div
                key={l.agent}
                class="pfgraph-lane-head"
                style={{ color: l.color, borderColor: `${l.color}66`, background: `${l.color}14` }}
                title={`${l.agent} · ${l.intervals.length} interval(s)`}
              >
                <span class="pfgraph-lane-head-dot" style={{ background: l.color }} />
                {l.agent}
                <span class="pfgraph-lane-head-count">{l.intervals.length}</span>
              </div>
            ))}
          </div>

          <div class="pfgraph-grid" style={{ height: `${graphHeight}px` }}>
            {parallelBands.map((band, i) => (
              <div
                key={`band-${i}`}
                class="pfgraph-parallel-band"
                style={{
                  top: `${(band.start / duration) * graphHeight}px`,
                  height: `${Math.max(4, ((band.end - band.start) / duration) * graphHeight)}px`,
                }}
                title={`${band.count} agents concurrent`}
                aria-hidden="true"
              />
            ))}

            {lanes.map((lane, col) => (
              <div
                key={lane.agent}
                class="pfgraph-lane"
                style={{ left: `calc((100% / ${lanes.length}) * ${col})`, width: `calc(100% / ${lanes.length})` }}
              >
                {lane.intervals.map((iv, i) => {
                  const end = iv.ongoing ? now : iv.end;
                  const top = ((iv.start - tMin) / duration) * graphHeight;
                  const height = Math.max(6, ((end - iv.start) / duration) * graphHeight);
                  const cls = iv.ongoing ? 'is-running' : 'is-done';
                  return (
                    <div
                      key={i}
                      class={`pfgraph-bar ${cls}`}
                      style={{
                        top: `${top}px`,
                        height: `${height}px`,
                        background: lane.color,
                        borderColor: `${lane.color}`,
                        boxShadow: iv.ongoing
                          ? `0 0 0 1px ${lane.color}44, 0 0 14px -2px ${lane.color}`
                          : undefined,
                      }}
                      title={`${lane.agent} · ${fmtDuration(end - iv.start)}${iv.ongoing ? ' (running)' : ''}`}
                    >
                      <span class="pfgraph-bar-tag">{fmtDuration(end - iv.start)}</span>
                      {iv.ongoing && <span class="pfgraph-bar-pulse" aria-hidden="true" />}
                    </div>
                  );
                })}
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

function buildLanes(events: BenderEvent[]): LaneData[] {
  const map = new Map<string, { intervals: Interval[]; openStart: number | null; firstStart: number }>();
  for (const ev of events) {
    const p = (ev.payload ?? {}) as Record<string, unknown>;
    const agent = typeof p.agent === 'string' ? p.agent : '';
    if (!agent) continue;
    const ts = Date.parse(ev.timestamp);
    if (!Number.isFinite(ts)) continue;

    let slot = map.get(agent);
    if (!slot) {
      slot = { intervals: [], openStart: null, firstStart: ts };
      map.set(agent, slot);
    }

    if (ev.type === 'agent_started') {
      slot.openStart = ts;
    } else if (ev.type === 'agent_completed' || ev.type === 'agent_failed' || ev.type === 'agent_blocked') {
      if (slot.openStart !== null) {
        slot.intervals.push({ start: slot.openStart, end: ts, ongoing: false });
        slot.openStart = null;
      }
    }
  }

  const out: LaneData[] = [];
  for (const [agent, slot] of map) {
    const intervals = [...slot.intervals];
    if (slot.openStart !== null) {
      intervals.push({ start: slot.openStart, end: slot.openStart, ongoing: true });
    }
    if (intervals.length === 0) continue;
    const total = intervals.reduce((acc, iv) => acc + Math.max(0, iv.end - iv.start), 0);
    out.push({
      agent,
      color: agentColor(agent),
      intervals,
      total,
      firstStart: slot.firstStart,
    });
  }
  out.sort((a, b) => a.firstStart - b.firstStart || a.agent.localeCompare(b.agent));
  return out;
}

function buildParallelBands(lanes: LaneData[], tMin: number, duration: number): Array<{ start: number; end: number; count: number }> {
  if (lanes.length === 0 || duration === 0) return [];
  interface Pt { t: number; delta: number }
  const points: Pt[] = [];
  for (const lane of lanes) {
    for (const iv of lane.intervals) {
      const end = iv.ongoing ? tMin + duration : iv.end;
      points.push({ t: iv.start - tMin, delta: 1 });
      points.push({ t: end - tMin, delta: -1 });
    }
  }
  points.sort((a, b) => a.t - b.t);

  const bands: Array<{ start: number; end: number; count: number }> = [];
  let active = 0;
  let bandStart: number | null = null;
  let lastT = 0;
  for (const p of points) {
    if (active >= 2 && bandStart !== null && p.t !== lastT) {
      // close running band at previous transition
    }
    if (active >= 2 && bandStart === null) bandStart = lastT;
    active += p.delta;
    if (active < 2 && bandStart !== null) {
      if (p.t > bandStart) bands.push({ start: bandStart, end: p.t, count: active + 1 });
      bandStart = null;
    }
    if (active >= 2 && bandStart === null) bandStart = p.t;
    lastT = p.t;
  }
  return bands;
}

function buildTicks(durationMs: number): Array<{ ms: number; label: string }> {
  if (durationMs <= 0) return [];
  let stepMs = 5_000;
  if (durationMs > 10 * 60_000) stepMs = 60_000;
  else if (durationMs > 5 * 60_000) stepMs = 30_000;
  else if (durationMs > 60_000) stepMs = 10_000;
  const out: Array<{ ms: number; label: string }> = [];
  for (let ms = 0; ms <= durationMs; ms += stepMs) {
    out.push({ ms, label: fmtTick(ms) });
  }
  return out;
}

function fmtTick(ms: number): string {
  if (ms === 0) return '0s';
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  const rem = s - m * 60;
  return rem ? `${m}m${rem.toString().padStart(2, '0')}s` : `${m}m`;
}

function fmtDuration(ms: number): string {
  if (ms < 1000) return `${Math.max(0, Math.round(ms))}ms`;
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(1)}s`;
  const m = Math.floor(s / 60);
  const rem = Math.floor(s - m * 60);
  return `${m}m${rem.toString().padStart(2, '0')}s`;
}

function useNow(live: boolean): number {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    if (!live) return;
    const id = setInterval(() => setNow(Date.now()), 500);
    return () => clearInterval(id);
  }, [live]);
  return now;
}

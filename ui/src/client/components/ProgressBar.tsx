import type { BenderEvent } from '../lib/api.ts';
import { agentColor } from '../lib/agents.ts';

interface Props {
  events: BenderEvent[];
  frozen: boolean;
  status?: string;
}

export function ProgressBar({ events, frozen, status }: Props) {
  let sessionPct = 0;
  let sessionStep = '';
  const perAgent = new Map<string, { pct: number; step: string }>();

  for (const ev of events) {
    if (ev.type === 'orchestrator_progress') {
      const p = (ev.payload ?? {}) as Record<string, unknown>;
      if (typeof p.percent === 'number') sessionPct = clamp(p.percent);
      if (typeof p.current_step === 'string') sessionStep = p.current_step;
    } else if (ev.type === 'agent_progress') {
      const p = (ev.payload ?? {}) as Record<string, unknown>;
      const agent = typeof p.agent === 'string' && p.agent ? p.agent : ev.actor.name;
      if (!agent) continue;
      const pct = typeof p.percent === 'number' ? clamp(p.percent) : 0;
      const step = typeof p.current_step === 'string' ? p.current_step : '';
      perAgent.set(agent, { pct, step });
    } else if (ev.type === 'session_completed') {
      sessionPct = 100;
      sessionStep = 'session_completed';
    }
  }

  if (frozen && sessionPct < 100) sessionPct = 100;

  const agents = [...perAgent.entries()].sort(([a], [b]) => a.localeCompare(b));
  const state = status === 'completed'
    ? 'done'
    : status === 'awaiting_confirm'
      ? 'awaits'
      : status === 'failed'
        ? 'failed'
        : sessionPct === 0
          ? 'idle'
          : 'live';

  return (
    <div class="progress-block">
      <div class="progress-dashboard">
        <div class="dial-frame">
          <span class="dial-tick dial-tick-tl" aria-hidden="true" />
          <span class="dial-tick dial-tick-tr" aria-hidden="true" />
          <span class="dial-tick dial-tick-bl" aria-hidden="true" />
          <span class="dial-tick dial-tick-br" aria-hidden="true" />
          <RadialDial percent={sessionPct} state={state} />
        </div>
        <div class="progress-readout">
          <div class="readout-line">
            <span class="readout-kicker">STAGE · telemetry</span>
            <StateSignal state={state} />
          </div>
          <div class="readout-step" title={sessionStep}>
            <span class="readout-caret" aria-hidden="true">›</span>
            <span class="readout-step-text">{sessionStep || (sessionPct === 0 ? 'standby — awaiting orchestrator' : '')}</span>
          </div>
          <div class="readout-meta">
            <MetaCell label="events" value={String(events.length).padStart(3, '0')} />
            <MetaCell label="agents" value={String(agents.length).padStart(2, '0')} />
            <MetaCell label="phase" value={state} />
          </div>
        </div>
      </div>
      {agents.length > 0 && (
        <div class="progress-agents">
          <div class="progress-agents-head">
            <span class="progress-agents-label">AGENTS</span>
            <span class="progress-agents-rule" />
          </div>
          {agents.map(([name, { pct, step }]) => {
            const color = agentColor(name);
            return (
              <div class="progress-row agent" key={name}>
                <div class="progress-head">
                  <span class="progress-label" style={{ color }}>{name}</span>
                  <span class="progress-step">{step}</span>
                  <span class="progress-pct">{pct}%</span>
                </div>
                <div class="progress-track">
                  <div class="progress-fill" style={{ width: `${pct}%`, background: color }} />
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

function RadialDial({ percent, state }: { percent: number; state: string }) {
  const size = 168;
  const stroke = 6;
  const radius = (size - stroke) / 2;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference * (1 - percent / 100);
  return (
    <div class="dial">
      <svg class="dial-svg" viewBox={`0 0 ${size} ${size}`} width={size} height={size} aria-hidden="true">
        <defs>
          <linearGradient id="dialFill" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stop-color="var(--signal)" />
            <stop offset="100%" stop-color="var(--phosphor)" />
          </linearGradient>
        </defs>
        <circle
          class="dial-track"
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke-width={stroke}
        />
        <circle
          class="dial-progress"
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke="url(#dialFill)"
          stroke-width={stroke}
          stroke-linecap="round"
          stroke-dasharray={circumference}
          stroke-dashoffset={offset}
          transform={`rotate(-90 ${size / 2} ${size / 2})`}
        />
        <g class={`dial-ticks ${state}`}>
          {Array.from({ length: 48 }).map((_, i) => {
            const a = (i / 48) * Math.PI * 2;
            const inner = radius - stroke * 2 - 4;
            const outer = radius - stroke - 2;
            const cx = size / 2;
            const cy = size / 2;
            return (
              <line
                key={i}
                x1={cx + Math.cos(a) * inner}
                y1={cy + Math.sin(a) * inner}
                x2={cx + Math.cos(a) * outer}
                y2={cy + Math.sin(a) * outer}
                stroke-width={i % 12 === 0 ? 1.5 : 0.6}
              />
            );
          })}
        </g>
      </svg>
      <div class="dial-readout">
        <div class="dial-number">
          <span class="dial-value">{percent}</span>
          <span class="dial-unit">%</span>
        </div>
        <div class="dial-caption">complete</div>
      </div>
    </div>
  );
}

function StateSignal({ state }: { state: string }) {
  return (
    <span class={`state-signal signal-${state}`}>
      <span class="state-signal-dot" aria-hidden="true" />
      {state}
    </span>
  );
}

function MetaCell({ label, value }: { label: string; value: string }) {
  return (
    <div class="meta-cell">
      <div class="meta-cell-key">{label}</div>
      <div class="meta-cell-val">{value}</div>
    </div>
  );
}

function clamp(n: number): number {
  if (!Number.isFinite(n)) return 0;
  if (n < 0) return 0;
  if (n > 100) return 100;
  return Math.round(n);
}

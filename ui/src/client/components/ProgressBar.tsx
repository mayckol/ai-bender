import type { BenderEvent } from '../lib/api.ts';
import { agentColor } from '../lib/agents.ts';

interface Props { events: BenderEvent[]; frozen: boolean; }

/**
 * Stage-level progress bar fed by `orchestrator_progress` events plus a
 * compact per-agent progress strip fed by `agent_progress` events. Renders
 * the latest percent seen for each emitter.
 */
export function ProgressBar({ events, frozen }: Props) {
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

  return (
    <div class="progress-block">
      <div class="progress-row session">
        <div class="progress-head">
          <span class="progress-label">Stage</span>
          <span class="progress-step">{sessionStep || (sessionPct === 0 ? 'waiting…' : '')}</span>
          <span class="progress-pct">{sessionPct}%</span>
        </div>
        <div class="progress-track">
          <div class="progress-fill session-fill" style={{ width: `${sessionPct}%` }} />
        </div>
      </div>
      {agents.length > 0 && (
        <div class="progress-agents">
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

function clamp(n: number): number {
  if (!Number.isFinite(n)) return 0;
  if (n < 0) return 0;
  if (n > 100) return 100;
  return Math.round(n);
}

import { useState } from 'preact/hooks';

import { agentColor, responsibleAgent } from '../lib/agents.ts';
import type { BenderEvent } from '../lib/api.ts';

interface Props { event: BenderEvent; }

export function EventRow({ event }: Props) {
  const ts = event.timestamp.length >= 19
    ? event.timestamp.slice(11, 19)
    : event.timestamp;
  const actorClass = `actor-${event.actor.kind}`;
  const agent = responsibleAgent(event);
  const color = agentColor(agent);
  const payloadSummary = summarize(event);

  return (
    <details class="event-row" data-agent={agent}>
      <summary class="row-summary">
        <span class="ts">{ts}</span>
        <span class="agent-badge" style={{ background: `${color}20`, color, borderColor: `${color}44` }}>
          {agent}
        </span>
        <span class={`type ${actorClass}`}>{event.type}</span>
        <span class="payload">{payloadSummary}</span>
      </summary>
      <EventDetail event={event} />
    </details>
  );
}

function EventDetail({ event }: { event: BenderEvent }) {
  if (event.type === 'file_changed') return <FileChangedDetail event={event} />;
  return <StructuredDetail event={event} />;
}

function StructuredDetail({ event }: { event: BenderEvent }) {
  const [raw, setRaw] = useState(false);
  const payload = event.payload ?? {};
  return (
    <div class="event-detail">
      <header class="event-detail-head">
        <EventTypeBadge event={event} />
        <span class="event-detail-rule" aria-hidden="true" />
        <button
          type="button"
          class="event-detail-toggle"
          onClick={() => setRaw((v) => !v)}
          title={raw ? 'Switch to structured view' : 'Switch to raw JSON'}
        >
          {raw ? '◈ structured' : '≡ raw JSON'}
        </button>
      </header>
      {raw ? (
        <pre class="event-detail-raw">{JSON.stringify(payload, null, 2)}</pre>
      ) : (
        <KVGrid payload={payload} />
      )}
    </div>
  );
}

function EventTypeBadge({ event }: { event: BenderEvent }) {
  const label = eventLabel(event);
  const tone = eventTone(event);
  return (
    <div class={`event-type-badge tone-${tone}`}>
      <span class="event-type-dot" aria-hidden="true" />
      <span class="event-type-text">{label}</span>
    </div>
  );
}

function eventLabel(event: BenderEvent): string {
  const p = (event.payload ?? {}) as Record<string, unknown>;
  if (event.type === 'orchestrator_decision' && typeof p.decision_type === 'string') {
    return `decision · ${p.decision_type}`;
  }
  if (event.type === 'skill_invoked' || event.type === 'skill_completed' || event.type === 'skill_failed') {
    const skill = typeof p.skill === 'string' ? p.skill : 'skill';
    return `${event.type.replace('skill_', 'skill · ')} · ${skill}`;
  }
  return event.type.replace(/_/g, ' ');
}

function eventTone(event: BenderEvent): 'signal' | 'phosphor' | 'err' | 'warn' | 'accent' | 'muted' {
  const p = (event.payload ?? {}) as Record<string, unknown>;
  if (event.type.endsWith('_failed') || event.type === 'agent_blocked') return 'err';
  if (event.type === 'finding_reported') {
    const sev = typeof p.severity === 'string' ? p.severity : '';
    if (sev === 'critical' || sev === 'high') return 'err';
    if (sev === 'medium') return 'warn';
    return 'accent';
  }
  if (event.type.endsWith('_completed')) return 'phosphor';
  if (event.type.endsWith('_started') || event.type === 'skill_invoked') return 'signal';
  if (event.type === 'orchestrator_decision') {
    const kind = typeof p.decision_type === 'string' ? p.decision_type : '';
    if (kind === 'parallel_dispatch_aborted' || kind === 'skip') return 'warn';
    if (kind === 'parallel_dispatch') return 'signal';
    return 'accent';
  }
  if (event.type === 'artifact_written' || event.type === 'file_changed') return 'phosphor';
  return 'muted';
}

/* ------------------------------------------------------------------ */
/* Structured key/value rendering                                      */
/* ------------------------------------------------------------------ */
function KVGrid({ payload }: { payload: unknown }) {
  const entries = Object.entries(payload as Record<string, unknown>);
  if (entries.length === 0) {
    return <div class="event-detail-empty">empty payload</div>;
  }
  return (
    <dl class="kv-grid">
      {entries.map(([key, value]) => (
        <div class="kv-row" key={key}>
          <dt class="kv-key">{key}</dt>
          <dd class="kv-val">
            <ValueCell keyName={key} value={value} />
          </dd>
        </div>
      ))}
    </dl>
  );
}

function ValueCell({ keyName, value }: { keyName: string; value: unknown }) {
  if (value === null || value === undefined || value === '') {
    return <span class="kv-muted">—</span>;
  }
  if (typeof value === 'boolean') {
    return <span class={`kv-bool kv-bool-${value}`}>{String(value)}</span>;
  }
  if (typeof value === 'number') {
    if (keyName.endsWith('_ms') || keyName === 'duration') {
      return <span class="kv-num">{formatMs(value)}</span>;
    }
    if (keyName === 'percent') {
      return <PercentBar value={value} />;
    }
    if (keyName.endsWith('_count') || keyName.endsWith('_total')) {
      return <span class="kv-num">{value.toLocaleString()}</span>;
    }
    return <span class="kv-num">{value}</span>;
  }
  if (typeof value === 'string') {
    if (keyName === 'severity') {
      return <span class={`kv-sev sev-${value}`}>{value}</span>;
    }
    if (keyName === 'status' || keyName === 'decision_type' || keyName === 'mode') {
      return <span class={`kv-token kv-token-${value}`}>{value}</span>;
    }
    if (keyName === 'agent' || keyName === 'dispatched_agent') {
      return <AgentChip name={value} />;
    }
    if (
      keyName === 'path' ||
      keyName === 'checksum' ||
      keyName === 'skill' ||
      keyName === 'command' ||
      keyName === 'working_dir' ||
      keyName.endsWith('_id') ||
      keyName === 'from_node' ||
      keyName === 'to_node' ||
      keyName === 'conflicting_path'
    ) {
      return <code class="kv-code">{value}</code>;
    }
    return <span class="kv-text">{value}</span>;
  }
  if (Array.isArray(value)) {
    if (value.length === 0) {
      return <span class="kv-muted">[]</span>;
    }
    if (keyName === 'dispatched_agents' || keyName === 'agents' || keyName === 'agents_summary' || keyName === 'fallback_order') {
      return (
        <div class="kv-chips">
          {value.map((v, i) => {
            if (typeof v === 'string') return <AgentChip key={i} name={v} />;
            if (v && typeof v === 'object' && typeof (v as Record<string, unknown>).agent === 'string') {
              return <AgentChip key={i} name={(v as Record<string, string>).agent} suffix={(v as Record<string, string>).status} />;
            }
            return <code key={i} class="kv-code">{JSON.stringify(v)}</code>;
          })}
        </div>
      );
    }
    if (keyName === 'node_ids' || keyName === 'task_ids' || keyName === 'skills_invoked' || keyName === 'inputs' || keyName === 'outputs' || keyName === 'source_artifacts' || keyName === 'tasks' || keyName === 'completed' || keyName === 'completed_nodes' || keyName === 'remaining_nodes' || keyName === 'skipped' || keyName === 'registered_projects') {
      return (
        <div class="kv-chips">
          {value.map((v, i) => (
            typeof v === 'string'
              ? <code key={i} class="kv-code-chip">{v}</code>
              : <code key={i} class="kv-code">{JSON.stringify(v)}</code>
          ))}
        </div>
      );
    }
    if (value.every((v) => typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean')) {
      return (
        <div class="kv-chips">
          {value.map((v, i) => <code key={i} class="kv-code-chip">{String(v)}</code>)}
        </div>
      );
    }
    return (
      <div class="kv-nested">
        {value.map((v, i) => (
          <div key={i} class="kv-nested-item">
            <span class="kv-nested-index">{i}</span>
            <KVGrid payload={v as Record<string, unknown>} />
          </div>
        ))}
      </div>
    );
  }
  if (typeof value === 'object') {
    return (
      <div class="kv-nested">
        <KVGrid payload={value as Record<string, unknown>} />
      </div>
    );
  }
  return <span class="kv-text">{String(value)}</span>;
}

function AgentChip({ name, suffix }: { name: string; suffix?: string }) {
  const color = agentColor(name);
  return (
    <span
      class="kv-agent-chip"
      style={{ background: `${color}1c`, color, borderColor: `${color}55` }}
    >
      <span class="kv-agent-chip-dot" style={{ background: color }} />
      {name}
      {suffix && <span class="kv-agent-chip-suffix">· {suffix}</span>}
    </span>
  );
}

function PercentBar({ value }: { value: number }) {
  const clamped = Math.max(0, Math.min(100, value));
  return (
    <div class="kv-percent">
      <div class="kv-percent-track">
        <div class="kv-percent-fill" style={{ width: `${clamped}%` }} />
      </div>
      <span class="kv-percent-num">{clamped}%</span>
    </div>
  );
}

function FileChangedDetail({ event }: { event: BenderEvent }) {
  const p = (event.payload ?? {}) as Record<string, unknown>;
  const path = typeof p.path === 'string' ? p.path : '(unknown)';
  const action = typeof p.action === 'string' ? p.action : undefined;
  const linesAdded = typeof p.lines_added === 'number' ? p.lines_added : 0;
  const linesRemoved = typeof p.lines_removed === 'number' ? p.lines_removed : 0;
  const preview = typeof p.preview === 'string' ? p.preview : '';
  const previewLines = typeof p.preview_line_count === 'number'
    ? p.preview_line_count
    : preview
      ? preview.split('\n').length
      : 0;
  const totalLines = typeof p.lines_total === 'number' ? p.lines_total : undefined;
  const more = totalLines !== undefined && previewLines > 0 && totalLines > previewLines
    ? totalLines - previewLines
    : 0;

  return (
    <div class="file-change">
      <div class="file-head">
        <span class={`file-action ${action ?? 'modify'}`}>{action ?? 'modify'}</span>
        <span class="file-path">{path}</span>
        <span class="file-stats">
          {linesAdded > 0 && <span class="added">+{linesAdded}</span>}
          {linesRemoved > 0 && <span class="removed">−{linesRemoved}</span>}
          {totalLines !== undefined && <span class="muted-small">{totalLines} total</span>}
        </span>
      </div>
      {preview ? (
        <pre class="file-preview">
          <code>{preview.replace(/\s+$/, '')}</code>
        </pre>
      ) : (
        <div class="muted-small" style={{ padding: '6px 10px' }}>No preview emitted by the agent.</div>
      )}
      {more > 0 && (
        <div class="file-more">… +{more} more lines (open the file on disk to view)</div>
      )}
    </div>
  );
}

function summarize(event: BenderEvent): string {
  const p = (event.payload ?? {}) as Record<string, unknown>;
  if (typeof p.error === 'string') return p.error;
  if (typeof p.skill === 'string') return p.skill;
  if (typeof p.dispatched_agent === 'string') return `→ ${p.dispatched_agent}`;
  if (Array.isArray(p.dispatched_agents) && p.dispatched_agents.length) {
    return `∥ ${(p.dispatched_agents as unknown[]).filter((x): x is string => typeof x === 'string').join(' + ')}`;
  }
  if (typeof p.path === 'string') return p.path;
  if (typeof p.title === 'string') return p.title;
  if (typeof p.decision_type === 'string') return String(p.decision_type);
  if (typeof p.stage === 'string') return p.stage;
  if (typeof p.status === 'string') return p.status;
  if (typeof p.command === 'string') return p.command;
  return '';
}

function formatMs(ms: number): string {
  if (ms < 1000) return `${ms} ms`;
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(2)} s`;
  const m = Math.floor(s / 60);
  const rem = s - m * 60;
  return `${m}m ${rem.toFixed(1)}s`;
}

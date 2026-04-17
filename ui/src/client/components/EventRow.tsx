import type { BenderEvent } from '../lib/api.ts';
import { agentColor, responsibleAgent } from '../lib/agents.ts';

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
      <summary class="row-summary" style={{ display: 'contents' }}>
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
  return (
    <div class="event-json">
      <pre style={{ margin: 0, fontSize: 11, overflowX: 'auto' }}>
        {JSON.stringify(event.payload ?? {}, null, 2)}
      </pre>
    </div>
  );
}

function FileChangedDetail({ event }: { event: BenderEvent }) {
  const p = (event.payload ?? {}) as Record<string, unknown>;
  const path = typeof p.path === 'string' ? p.path : '(unknown)';
  const action = typeof p.action === 'string' ? p.action : undefined; // "create" | "modify" | "delete"
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
  const p = event.payload ?? {};
  if ('error' in p) return String((p as any).error);
  if ('skill' in p) return String((p as any).skill);
  if ('dispatched_agent' in p) return `→ ${(p as any).dispatched_agent}`;
  if ('path' in p) return String((p as any).path);
  if ('title' in p) return String((p as any).title);
  if ('decision_type' in p) return String((p as any).decision_type);
  if ('stage' in p) return String((p as any).stage);
  if ('status' in p) return String((p as any).status);
  if ('command' in p) return String((p as any).command);
  return '';
}

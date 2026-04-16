import type { BenderEvent } from '../lib/api.ts';

interface Props { event: BenderEvent; }

export function EventRow({ event }: Props) {
  const ts = event.timestamp.length >= 19
    ? event.timestamp.slice(11, 19)
    : event.timestamp;
  const actorClass = `actor-${event.actor.kind}`;
  const payloadSummary = summarize(event);

  return (
    <details class="event-row">
      <summary class="row-summary" style={{ display: 'contents' }}>
        <span class="ts">{ts}</span>
        <span class={`type ${actorClass}`}>
          {event.type}
          <span class="actor"> · {event.actor.name}</span>
        </span>
        <span class="payload">{payloadSummary}</span>
      </summary>
      <div style={{ gridColumn: '1 / -1', padding: '6px 10px', background: 'var(--panel-2)' }}>
        <pre style={{ margin: 0, fontSize: 11, overflowX: 'auto' }}>
          {JSON.stringify(event.payload ?? {}, null, 2)}
        </pre>
      </div>
    </details>
  );
}

function summarize(event: BenderEvent): string {
  const p = event.payload ?? {};
  // Prefer the most informative single field per event type.
  if ('error' in p) return String((p as any).error);
  if ('skill' in p) return String((p as any).skill);
  if ('agent' in p && 'task_ids' in p) return `${(p as any).agent} [${(p as any).task_ids}]`;
  if ('agent' in p) return String((p as any).agent);
  if ('path' in p) return String((p as any).path);
  if ('title' in p) return String((p as any).title);
  if ('decision_type' in p) return String((p as any).decision_type);
  if ('stage' in p) return String((p as any).stage);
  if ('status' in p) return String((p as any).status);
  if ('command' in p) return String((p as any).command);
  return '';
}

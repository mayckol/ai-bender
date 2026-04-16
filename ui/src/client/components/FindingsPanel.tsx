import type { BenderEvent } from '../lib/api.ts';

interface Props { events: BenderEvent[]; }

interface Finding {
  id: string;
  severity: string;
  title: string;
  category?: string;
}

export function FindingsPanel({ events }: Props) {
  const findings: Finding[] = [];
  for (const ev of events) {
    if (ev.type !== 'finding_reported') continue;
    const p = (ev.payload ?? {}) as Record<string, unknown>;
    findings.push({
      id: String(p.finding_id ?? ''),
      severity: String(p.severity ?? 'info'),
      category: p.category ? String(p.category) : undefined,
      title: String(p.title ?? p.description ?? '(no title)'),
    });
  }

  if (findings.length === 0) {
    return (
      <div class="card">
        <h2>Findings</h2>
        <div class="empty">No findings reported yet.</div>
      </div>
    );
  }

  return (
    <div class="card">
      <h2>Findings ({findings.length})</h2>
      <ul class="findings-list">
        {findings.map((f) => (
          <li key={f.id}>
            <span class={`severity ${f.severity}`}>{f.severity}</span>
            <strong>{f.id}</strong>
            {f.category && <span class="breadcrumb"> · {f.category}</span>}
            <div>{f.title}</div>
          </li>
        ))}
      </ul>
    </div>
  );
}
